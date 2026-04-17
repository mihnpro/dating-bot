import logging
from typing import Optional

from ..domain.profile import Profile
from ..domain.user import User
from ..infrastructure.user_profile_client import UserProfileClient

logger = logging.getLogger(__name__)

OPPOSITE_GENDER: dict[str, str] = {"male": "female", "female": "male"}


class UserUseCases:
    """Application-level use cases that orchestrate user and profile operations."""

    def __init__(self, client: UserProfileClient) -> None:
        self._client = client

    # ------------------------------------------------------------------ #
    # User                                                                 #
    # ------------------------------------------------------------------ #

    async def get_or_register_user(
        self,
        telegram_id: int,
        username: str,
        first_name: str,
        last_name: str,
    ) -> User:
        """Return existing user or register a new one."""
        existing = await self._client.get_user_by_telegram_id(telegram_id)
        if existing:
            return User.from_dict(existing)

        logger.info("Registering new user telegram_id=%s", telegram_id)
        registered = await self._client.register_user(
            telegram_id=telegram_id,
            username=username,
            first_name=first_name,
            last_name=last_name,
        )
        return User.from_dict(registered)

    async def get_user(self, user_id: int) -> Optional[User]:
        """Fetch a user by internal ID. Returns None if not found."""
        data = await self._client.get_user(user_id)
        return User.from_dict(data) if data else None

    # ------------------------------------------------------------------ #
    # Profile                                                              #
    # ------------------------------------------------------------------ #

    async def get_profile(self, user_id: int) -> Optional[Profile]:
        """Fetch a user's profile. Returns None if the profile does not exist yet."""
        data = await self._client.get_profile(user_id)
        return Profile.from_dict(data) if data else None

    async def create_profile(
        self,
        user_id: int,
        age: int,
        gender: str,
        city: str,
        interests: list[str],
    ) -> Profile:
        """Create a new profile for the given user."""
        logger.info("Creating profile for user_id=%s", user_id)
        data = await self._client.create_profile(
            user_id=user_id,
            age=age,
            gender=gender,
            city=city,
            interests=interests,
        )
        return Profile.from_dict(data)

    async def update_profile(self, user_id: int, **kwargs) -> Profile:
        """Partially update a user's profile. Pass only the fields to change."""
        logger.info(
            "Updating profile for user_id=%s fields=%s", user_id, list(kwargs.keys())
        )
        data = await self._client.update_profile(user_id=user_id, **kwargs)
        return Profile.from_dict(data)

    # ------------------------------------------------------------------ #
    # Browse feed                                                          #
    # ------------------------------------------------------------------ #

    async def get_profiles_for_browse(
        self,
        current_user_id: int,
        opposite_gender: str,
        page: int = 1,
        page_size: int = 5,
    ) -> list[Profile]:
        """
        Return a page of profiles with the opposite gender, excluding the
        current user's own profile.

        NOTE: Recommendation Service is not yet implemented; we fall back
        to a plain ListProfiles call filtered by gender.
        """
        data = await self._client.list_profiles(
            page=page,
            page_size=page_size,
            gender=opposite_gender,
        )
        profiles_raw: list[dict] = data.get("profiles", [])
        profiles = [Profile.from_dict(p) for p in profiles_raw]
        # Exclude own profile in case it somehow appears in the feed
        return [p for p in profiles if p.user_id != current_user_id]
