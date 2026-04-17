import asyncio
import logging

from aiogram.types import BotCommand

from .application.matching_use_cases import MatchingUseCases
from .application.user_use_cases import UserUseCases
from .bot import bot, dp
from .config import settings
from .infrastructure.matching_client import MatchingClient
from .infrastructure.recommendation_client import RecommendationClient
from .infrastructure.user_profile_client import UserProfileClient
from .presentation.routers import matching as matching_router
from .presentation.routers import profile, start

logging.basicConfig(
    level=logging.INFO,
    format="%(asctime)s | %(levelname)-8s | %(name)s — %(message)s",
)
logger = logging.getLogger(__name__)


async def _set_bot_commands() -> None:
    commands = [
        BotCommand(command="start", description="Start the bot"),
        BotCommand(command="createprofile", description="Create your profile"),
        BotCommand(command="profile", description="View your profile"),
        BotCommand(command="edit", description="Edit your profile"),
        BotCommand(command="browse", description="Browse profiles"),
        BotCommand(command="matches", description="View your matches"),
        BotCommand(command="help", description="Show help"),
    ]
    await bot.set_my_commands(commands)
    logger.info("Bot commands registered")


async def main() -> None:
    logger.info("Starting Gateway Service (%s)", settings.service_name)

    # ------------------------------------------------------------------ #
    # Infrastructure — HTTP clients for Go microservices                  #
    # ------------------------------------------------------------------ #
    user_profile_client = UserProfileClient(settings.user_profile_service_url)
    matching_client = MatchingClient(settings.matching_service_url)
    recommendation_client = RecommendationClient(settings.recommendation_service_url)

    await user_profile_client.start()
    await matching_client.start()
    await recommendation_client.start()

    # ------------------------------------------------------------------ #
    # Application — use-case layer                                        #
    # ------------------------------------------------------------------ #
    user_use_cases = UserUseCases(user_profile_client)
    matching_use_cases = MatchingUseCases(matching_client, user_profile_client)

    # ------------------------------------------------------------------ #
    # Dependency injection via dispatcher workflow data                   #
    # Aiogram 3.x passes dp["key"] values as handler kwargs automatically #
    # ------------------------------------------------------------------ #
    dp["user_use_cases"] = user_use_cases
    dp["matching_use_cases"] = matching_use_cases
    dp["recommendation_client"] = recommendation_client

    # ------------------------------------------------------------------ #
    # Routers                                                             #
    # ------------------------------------------------------------------ #
    dp.include_router(start.router)
    dp.include_router(profile.router)
    dp.include_router(matching_router.router)

    # ------------------------------------------------------------------ #
    # Bot commands visible in Telegram UI                                 #
    # ------------------------------------------------------------------ #
    await _set_bot_commands()

    # ------------------------------------------------------------------ #
    # Start polling                                                       #
    # ------------------------------------------------------------------ #
    try:
        logger.info("Bot is polling for updates...")
        await dp.start_polling(
            bot,
            allowed_updates=dp.resolve_used_update_types(),
        )
    finally:
        logger.info("Shutting down Gateway Service...")
        await user_profile_client.stop()
        await matching_client.stop()
        await recommendation_client.stop()
        await bot.session.close()
        logger.info("Gateway Service stopped.")


if __name__ == "__main__":
    asyncio.run(main())
