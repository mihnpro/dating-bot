import asyncio
import logging

from aiohttp import web as aiohttp_web
from aiogram.types import BotCommand

from .application.matching_use_cases import MatchingUseCases
from .application.media_use_cases import MediaUseCases
from .application.user_use_cases import UserUseCases
from .bot import bot, dp
from .config import settings
from .infrastructure.chat_client import ChatClient
from .infrastructure.matching_client import MatchingClient
from .infrastructure.media_client import MediaClient
from .infrastructure.recommendation_client import RecommendationClient
from .infrastructure.user_profile_client import UserProfileClient
from .presentation.routers import matching as matching_router
from .presentation.routers import media as media_router
from .presentation.routers import profile, start
from .presentation.routers.notify import create_internal_app

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
        BotCommand(command="photos", description="Manage your photos"),
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
    media_client = MediaClient(settings.media_service_url, settings.minio_internal_url)
    chat_client = ChatClient(settings.chat_service_url)

    await user_profile_client.start()
    await matching_client.start()
    await recommendation_client.start()
    await media_client.start()
    await chat_client.start()

    # ------------------------------------------------------------------ #
    # Application — use-case layer                                        #
    # ------------------------------------------------------------------ #
    user_use_cases = UserUseCases(user_profile_client)
    matching_use_cases = MatchingUseCases(matching_client, user_profile_client)
    media_use_cases = MediaUseCases(media_client, bot)

    # ------------------------------------------------------------------ #
    # Dependency injection via dispatcher workflow data                   #
    # Aiogram 3.x passes dp["key"] values as handler kwargs automatically #
    # ------------------------------------------------------------------ #
    dp["user_use_cases"] = user_use_cases
    dp["matching_use_cases"] = matching_use_cases
    dp["recommendation_client"] = recommendation_client
    dp["media_use_cases"] = media_use_cases
    dp["chat_client"] = chat_client
    dp["chat_frontend_url"] = settings.chat_frontend_url

    # ------------------------------------------------------------------ #
    # Routers                                                             #
    # ------------------------------------------------------------------ #
    dp.include_router(start.router)
    dp.include_router(profile.router)
    dp.include_router(matching_router.router)
    dp.include_router(media_router.router)

    # ------------------------------------------------------------------ #
    # Bot commands visible in Telegram UI                                 #
    # ------------------------------------------------------------------ #
    await _set_bot_commands()

    # ------------------------------------------------------------------ #
    # Internal HTTP server for notification-service delivery              #
    # Listens on INTERNAL_HTTP_PORT (default 8086).                       #
    # POST /internal/notify → bot.send_message(telegram_id, text)        #
    # ------------------------------------------------------------------ #
    internal_app = create_internal_app(bot)
    runner = aiohttp_web.AppRunner(internal_app)
    await runner.setup()
    site = aiohttp_web.TCPSite(runner, host="0.0.0.0", port=settings.internal_http_port)
    await site.start()
    logger.info("Internal HTTP server started on port %d", settings.internal_http_port)

    # ------------------------------------------------------------------ #
    # Start polling alongside the internal HTTP server                    #
    # ------------------------------------------------------------------ #
    try:
        logger.info("Bot is polling for updates...")
        await dp.start_polling(
            bot,
            allowed_updates=dp.resolve_used_update_types(),
        )
    finally:
        logger.info("Shutting down Gateway Service...")
        await runner.cleanup()
        await user_profile_client.stop()
        await matching_client.stop()
        await recommendation_client.stop()
        await media_client.stop()
        await chat_client.stop()
        await bot.session.close()
        logger.info("Gateway Service stopped.")


if __name__ == "__main__":
    asyncio.run(main())