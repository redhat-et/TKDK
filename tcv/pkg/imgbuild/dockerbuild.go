/*
Copyright Red Hat Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

	http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/
package imgbuild

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/archive"
	"github.com/redhat-et/TKDK/tcv/pkg/constants"
	"github.com/redhat-et/TKDK/tcv/pkg/preflightcheck"
	"github.com/redhat-et/TKDK/tcv/pkg/utils"
	logging "github.com/sirupsen/logrus"
)

type dockerBuilder struct{}

// Docker implementation of the ImageBuilder interface.
func (d *dockerBuilder) CreateImage(imageName, cacheDir string) error {
	dockerfilePath := fmt.Sprintf("%s/Dockerfile", constants.TCVTmpDir)
	tmpCacheDir := fmt.Sprintf("%s/io.triton.cache", constants.TCVTmpDir)
	tmpManifestDir := fmt.Sprintf("%s/io.triton.manifest", constants.TCVTmpDir)
	var allMetadata []CacheMetadata

	// Copy cache contents into a directory within build context
	if err := os.MkdirAll(tmpCacheDir, 0755); err != nil {
		return fmt.Errorf("failed to create temp cache dir: %w", err)
	}
	defer os.RemoveAll(tmpCacheDir)

	err := copyDir(cacheDir+"/.", tmpCacheDir)
	if err != nil {
		return fmt.Errorf("failed to copy cacheDir into build context: %w", err)
	}

	totalSize, err := getTotalDirSize(tmpCacheDir)
	if err != nil {
		return fmt.Errorf("failed to compute total cache size: %w", err)
	}

	jsonFiles, err := preflightcheck.FindAllTritonCacheJSON(tmpCacheDir)
	if err != nil {
		return fmt.Errorf("failed to find cache files: %w", err)
	}

	for _, jsonFile := range jsonFiles {
		data, ret := preflightcheck.GetTritonCacheJSONData(jsonFile)
		if ret != nil {
			return fmt.Errorf("failed to extract data from %s: %w", jsonFile, ret)
		}
		if data == nil {
			continue
		}

		dummyKey, ret := preflightcheck.ComputeDummyTritonKey(data)
		if ret != nil {
			return fmt.Errorf("failed to calculate dummy triton key for %s: %w", jsonFile, ret)
		}

		allMetadata = append(allMetadata, CacheMetadata{
			Hash:       data.Hash,
			Backend:    data.Target.Backend,
			Arch:       preflightcheck.ConvertArchToString(data.Target.Arch),
			WarpSize:   data.Target.WarpSize,
			PTXVersion: data.PtxVersion,
			NumStages:  data.NumStages,
			NumWarps:   data.NumWarps,
			Debug:      data.Debug,
			DummyKey:   dummyKey,
		})
	}

	filepath.Walk(tmpCacheDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() && strings.HasPrefix(info.Name(), "__grp__") && strings.HasSuffix(info.Name(), ".json") {
			if err := utils.SanitizeGroupJSON(path); err != nil {
				logging.Warnf("could not sanitize %s: %v", path, err)
			}
		}
		return nil
	})

	// Ensure manifest directory exists
	if err = os.MkdirAll(tmpManifestDir, 0755); err != nil {
		return fmt.Errorf("failed to create manifest directory: %w", err)
	}

	manifestPath := filepath.Join(tmpManifestDir, "manifest.json")
	err = writeCacheManifest(manifestPath, allMetadata)
	if err != nil {
		return fmt.Errorf("failed to write manifest: %w", err)
	}

	err = generateDockerfile(imageName, constants.DockerfileCacheDir, constants.DockerfileManifestDir, dockerfilePath)
	if err != nil {
		return fmt.Errorf("failed to generate Dockerfile: %w", err)
	}
	defer os.Remove(dockerfilePath)

	if _, err = os.Stat(dockerfilePath); os.IsNotExist(err) {
		return fmt.Errorf("dockerfile not found at %s", dockerfilePath)
	}

	apiClient, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return fmt.Errorf("failed to create Docker client: %w", err)
	}

	tar, err := archive.TarWithOptions(constants.TCVTmpDir, &archive.TarOptions{IncludeSourceDir: false})
	if err != nil {
		return fmt.Errorf("error creating tar: %w", err)
	}
	defer tar.Close()

	summary, err := BuildTritonSummary(allMetadata)
	if err != nil {
		return fmt.Errorf("failed to build image summary: %w", err)
	}

	summaryJSON, err := json.Marshal(summary)
	if err != nil {
		return fmt.Errorf("failed to marshal summary for label: %w", err)
	}

	labels := map[string]string{
		"cache.triton.image/summary":          string(summaryJSON),
		"cache.triton.image/entry-count":      strconv.Itoa(len(allMetadata)),
		"cache.triton.image/cache-size-bytes": strconv.FormatInt(totalSize, 10),
	}
	buildOptions := types.ImageBuildOptions{
		Dockerfile: "Dockerfile",
		Tags:       []string{imageName},
		NoCache:    true,
		Remove:     false,
		Labels:     labels,
	}

	buildResponse, err := apiClient.ImageBuild(context.Background(), tar, buildOptions)
	if err != nil {
		return fmt.Errorf("error building image: %w", err)
	}
	defer buildResponse.Body.Close()

	_, err = io.Copy(os.Stdout, buildResponse.Body)
	if err != nil {
		return fmt.Errorf("error reading build output: %w", err)
	}

	imageWithTag := imageName
	if !strings.Contains(imageName, ":") {
		imageWithTag = fmt.Sprintf("%s:latest", imageName)
	}

	err = apiClient.ImageTag(context.Background(), imageName, imageWithTag)
	if err != nil {
		return fmt.Errorf("error tagging image: %w", err)
	}

	ret := utils.CleanupTCVDirs()
	if ret != nil {
		return fmt.Errorf("could not cleanup tmp dirs %v", ret)
	}
	logging.Info("Docker image built successfully")
	return nil
}
