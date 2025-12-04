"""ErsatzTV API client for smart collection management."""

from dataclasses import dataclass

import httpx


@dataclass
class Channel:
    """Represents an ErsatzTV channel."""

    id: int
    number: str
    name: str


@dataclass
class SmartCollection:
    """Represents an ErsatzTV smart collection."""

    id: int
    name: str
    query: str


class ErsatzTVClient:
    """Client for ErsatzTV REST API."""

    def __init__(self, base_url: str):
        self.base_url = base_url.rstrip("/")
        self.client = httpx.Client(timeout=30.0)

    def _get(self, endpoint: str) -> list | dict:
        """Make a GET request to the API."""
        url = f"{self.base_url}{endpoint}"
        response = self.client.get(url)
        response.raise_for_status()
        return response.json()

    def _post(self, endpoint: str, data: dict) -> dict:
        """Make a POST request to the API."""
        url = f"{self.base_url}{endpoint}"
        response = self.client.post(url, json=data)
        if response.status_code >= 400:
            print(f"POST {endpoint} failed: {response.status_code}")
            print(f"Response: {response.text}")
        response.raise_for_status()
        return response.json()

    def _put(self, endpoint: str, data: dict) -> dict:
        """Make a PUT request to the API."""
        url = f"{self.base_url}{endpoint}"
        response = self.client.put(url, json=data)
        response.raise_for_status()
        return response.json()

    def _delete(self, endpoint: str) -> None:
        """Make a DELETE request to the API."""
        url = f"{self.base_url}{endpoint}"
        response = self.client.delete(url)
        response.raise_for_status()

    def get_channels(self) -> list[Channel]:
        """Get all channels."""
        data = self._get("/api/channels")
        return [
            Channel(
                id=item.get("id", 0),
                number=item.get("number", ""),
                name=item.get("name", ""),
            )
            for item in data
        ]

    def get_smart_collections(self) -> list[SmartCollection]:
        """Get all smart collections."""
        data = self._get("/api/collections/smart")
        return [
            SmartCollection(
                id=item.get("id", 0),
                name=item.get("name", ""),
                query=item.get("query", ""),
            )
            for item in data
        ]

    def create_smart_collection(self, name: str, query: str) -> SmartCollection | None:
        """Create a new smart collection."""
        try:
            # ErsatzTV API uses /api/collections/smart/new for creating
            # and expects Query and Name (capitalized)
            data = self._post(
                "/api/collections/smart/new",
                {"Name": name, "Query": query},
            )
            return SmartCollection(
                id=data.get("id", 0),
                name=data.get("name", name),
                query=data.get("query", query),
            )
        except Exception as e:
            print(f"Error creating smart collection: {e}")
            return None

    def update_smart_collection(
        self, collection_id: int, name: str, query: str
    ) -> SmartCollection | None:
        """Update an existing smart collection."""
        try:
            data = self._put(
                f"/api/collections/smart/{collection_id}",
                {"id": collection_id, "name": name, "query": query},
            )
            return SmartCollection(
                id=data.get("id", collection_id),
                name=data.get("name", name),
                query=data.get("query", query),
            )
        except Exception as e:
            print(f"Error updating smart collection: {e}")
            return None

    def delete_smart_collection(self, collection_id: int) -> bool:
        """Delete a smart collection."""
        try:
            self._delete(f"/api/collections/smart/{collection_id}")
            return True
        except Exception as e:
            print(f"Error deleting smart collection: {e}")
            return False

    def close(self) -> None:
        """Close the HTTP client."""
        self.client.close()

    def __enter__(self) -> "ErsatzTVClient":
        return self

    def __exit__(self, *args) -> None:
        self.close()
