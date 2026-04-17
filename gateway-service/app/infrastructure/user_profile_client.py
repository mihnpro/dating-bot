import logging
from typing import Optional

from .base_client import BaseClient

logger = logging.getLogger(__name__)


class UserProfileClient(BaseClient):
    """HTTP client for user-profile-service via grpc-gateway REST endpoints."""

    async def register_user(
        self,
        telegram_id: int,
        username: str,
        first_name: str,
        last_name: str,
        referral_by: Optional[int] = None,
    ) -> dict:
        payload: dict = {
            "telegramId": telegram_id,
            "username": username,
            "firstName": first_name,
            "lastName": last_name,
        }
        if referral_by is not None:
            payload["referralBy"] = referral_by
        return await self._post("/api/v1/users/register", json=payload)

    async def get_user_by_telegram_id(self, telegram_id: int) -> Optional[dict]:
        try:
            return await self._get(f"/api/v1/users/telegram/{telegram_id}")
        except Exception as exc:
            logger.warning("get_user_by_telegram_id(%s) failed: %s", telegram_id, exc)
            return None

    async def get_user(self, user_id: int) -> Optional[dict]:
        try:
            return await self._get(f"/api/v1/users/{user_id}")
        except Exception as exc:
            logger.warning("get_user(%s) failed: %s", user_id, exc)
            return None

    async def create_profile(
        self,
        user_id: int,
        age: int,
        gender: str,
        city: str,
        interests: list[str],
    ) -> dict:
        payload = {
            "userId": user_id,
            "age": age,
            "gender": gender,
            "city": city,
            "interests": interests,
        }
        return await self._post("/api/v1/profiles", json=payload)

    async def get_profile(self, user_id: int) -> Optional[dict]:
        try:
            return await self._get(f"/api/v1/profiles/{user_id}")
        except Exception as exc:
            logger.warning("get_profile(%s) failed: %s", user_id, exc)
            return None

    async def update_profile(self, user_id: int, **kwargs) -> dict:
        payload = {k: v for k, v in kwargs.items() if v is not None}
        return await self._patch(f"/api/v1/profiles/{user_id}", json=payload)

    async def list_profiles(
        self,
        page: int = 1,
        page_size: int = 10,
        gender: Optional[str] = None,
        city: Optional[str] = None,
        min_age: Optional[int] = None,
        max_age: Optional[int] = None,
    ) -> dict:
        params: dict = {"page": page, "pageSize": page_size}
        if gender is not None:
            params["gender"] = gender
        if city is not None:
            params["city"] = city
        if min_age is not None:
            params["minAge"] = min_age
        if max_age is not None:
            params["maxAge"] = max_age
        return await self._get("/api/v1/profiles", params=params)
