"""
Service for search Triton kernels based on criteria.
"""

from __future__ import annotations
from typing import Any, Dict, List
from ..data.database import Database
from ..models.criteria import SearchCriteria


class SearchService:
    """
    Search Triton kernels based on filters
    """

    def __init__(self, criteria: SearchCriteria):
        """
        Initialize the search service.
        """
        self.db = Database()
        self.criteria = criteria

    def search(self) -> List[Dict[str, Any]]:
        """
        Searches for kernels matching criteria.

        Returns:
            A list of dictionaries, each representing a matching kernel.
        """
        found = self.db.search(self.criteria)

        return found

    def close(self):
        """Close the database connection."""
        self.db.close()
