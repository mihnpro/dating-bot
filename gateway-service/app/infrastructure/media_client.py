import logging
from typing import Optional
from urllib.parse import urlparse

import aiohttp

from .base_client import BaseClient

logger = logging.getLogger(__name__)


class MediaClient(BaseClient):
    def __init__(self, base_url: str, minio_internal_url: str = "http://minio:9000") -> None:
        super().__init__(base_url)
        self._minio_internal_url = minio_internal_url.rstrip("/")
    """HTTP client for media-service.

    All endpoints except upload use JSON. Upload uses multipart/form-data
    because grpc-gateway cannot handle binary streams — the media-service
    exposes a dedicated native HTTP handler for that endpoint.
    """

    async def upload_photo(
        self,
        user_id: int,
        filename: str,
        content: bytes,
        content_type: str = "image/jpeg",
    ) -> Optional[dict]:
        """Upload a photo as multipart/form-data. Returns the created MediaItem dict."""
        url = f"{self.base_url}/api/v1/media/upload"
        form = aiohttp.FormData()
        form.add_field("user_id", str(user_id))
        form.add_field(
            "file",
            content,
            filename=filename,
            content_type=content_type,
        )
        try:
            async with self.session.post(url, data=form) as resp:
                if resp.status == 201:
                    return await resp.json()
                body = await resp.text()
                logger.error("upload_photo failed status=%s body=%s", resp.status, body)
                return None
        except Exception as exc:
            logger.error("upload_photo exception: %s", exc)
            return None

    async def get_user_photos(self, user_id: int) -> list[dict]:
        """Return all photos for a user ordered by main first."""
        try:
            data = await self._get(f"/api/v1/media/user/{user_id}")
            # handler returns a JSON array directly
            if isinstance(data, list):
                return data
            return []
        except Exception as exc:
            logger.warning("get_user_photos(%s) failed: %s", user_id, exc)
            return []

    async def delete_photo(self, media_id: int, user_id: int) -> bool:
        """Delete a photo. Returns True on success."""
        url = f"{self.base_url}/api/v1/media/{media_id}"
        try:
            async with self.session.delete(url, params={"user_id": str(user_id)}) as resp:
                return resp.status in (200, 204)
        except Exception as exc:
            logger.warning("delete_photo(%s) failed: %s", media_id, exc)
            return False

    async def set_main_photo(self, media_id: int, user_id: int) -> bool:
        """Mark a photo as the main (cover) photo."""
        url = f"{self.base_url}/api/v1/media/{media_id}/main"
        try:
            async with self.session.patch(url, json={"user_id": user_id}) as resp:
                return resp.status in (200, 204)
        except Exception as exc:
            logger.warning("set_main_photo(%s) failed: %s", media_id, exc)
            return False

    async def download_bytes(self, public_url: str) -> Optional[bytes]:
        """Download photo bytes via the internal MinIO address.

        The public URL (e.g. http://localhost:9000/photos/key) is rewritten to
        the Docker-internal MinIO address so the gateway can fetch it directly.
        """
        path = urlparse(public_url).path  # /photos/key
        internal_url = f"{self._minio_internal_url}{path}"
        try:
            async with self.session.get(internal_url) as resp:
                if resp.status == 200:
                    return await resp.read()
                logger.warning("download_bytes status=%s url=%s", resp.status, internal_url)
                return None
        except Exception as exc:
            logger.warning("download_bytes failed url=%s: %s", internal_url, exc)
            return None
