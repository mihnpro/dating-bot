import logging
from typing import Optional

from aiogram import Bot

from ..domain.media import MediaItem
from ..infrastructure.media_client import MediaClient

logger = logging.getLogger(__name__)

MAX_PHOTOS = 6


class MediaUseCases:
    """Orchestrates photo upload flow:
    Telegram file → download bytes → upload to media-service.
    """

    def __init__(self, client: MediaClient, bot: Bot) -> None:
        self._client = client
        self._bot = bot

    async def get_user_photos(self, user_id: int) -> list[MediaItem]:
        items = await self._client.get_user_photos(user_id)
        return [MediaItem.from_dict(i) for i in items]

    async def upload_from_telegram(
        self,
        user_id: int,
        file_id: str,
        mime_type: str = "image/jpeg",
    ) -> Optional[MediaItem]:
        """Download photo from Telegram servers and upload to media-service."""
        photos = await self._client.get_user_photos(user_id)
        if len(photos) >= MAX_PHOTOS:
            return None  # caller checks for None and shows limit message

        try:
            tg_file = await self._bot.get_file(file_id)
            file_bytes = await self._bot.download_file(tg_file.file_path)
            content = file_bytes.read()
        except Exception as exc:
            logger.error("Failed to download file %s from Telegram: %s", file_id, exc)
            return None

        ext = "jpg" if "jpeg" in mime_type else mime_type.split("/")[-1]
        filename = f"{file_id}.{ext}"

        data = await self._client.upload_photo(
            user_id=user_id,
            filename=filename,
            content=content,
            content_type=mime_type,
        )
        if data is None:
            return None

        return MediaItem.from_dict(data)

    async def delete_photo(self, media_id: int, user_id: int) -> bool:
        return await self._client.delete_photo(media_id, user_id)

    async def set_main_photo(self, media_id: int, user_id: int) -> bool:
        return await self._client.set_main_photo(media_id, user_id)

    async def get_photo_bytes(self, url: str) -> Optional[bytes]:
        return await self._client.download_bytes(url)
