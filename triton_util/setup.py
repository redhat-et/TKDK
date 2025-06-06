"""Setup script for the triton-util package."""

from setuptools import setup, find_packages

with open("README.md", encoding="utf-8") as f:
    long_description = f.read()

setup(
    name="triton-util",
    version="0.0.2",
    packages=find_packages(),
    install_requires=["triton"],
    extras_require={
        "dev": ["pytest", "pylint", "torch", "ipython"],
    },
    author="Umer Adil",
    author_email="umer.hayat.adil@gmail.com",
    description="Make Triton easier - A utility package for OpenAI Triton",
    long_description=long_description,
    long_description_content_type="text/markdown",
    url="https://github.com/redhat-et/TKDK/triton_util",
    classifiers=[
        "Programming Language :: Python :: 3",
        "License :: OSI Approved :: MIT License",
        "Operating System :: OS Independent",
    ],
    python_requires=">=3.12",
)
