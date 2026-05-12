import logging

from aiohttp import web
from aiogram import Bot

logger = logging.getLogger(__name__)


async def handle_notify(request: web.Request) -> web.Response:
    """POST /internal/notify — delivers a Telegram message on behalf of a microservice.

    Expected JSON body:
        { "telegram_id": <int>, "text": "<string>" }
    """
    try:
        data = await request.json()
    except Exception:
        return web.json_response({"error": "invalid JSON"}, status=400)

    telegram_id: int | None = data.get("telegram_id")
    text: str | None = data.get("text")

    if not telegram_id or not text:
        return web.json_response({"error": "telegram_id and text are required"}, status=400)

    bot: Bot = request.app["bot"]

    try:
        await bot.send_message(chat_id=telegram_id, text=text)
        logger.info("notification delivered telegram_id=%s", telegram_id)
        return web.json_response({"ok": True})
    except Exception as exc:
        logger.warning("failed to deliver notification telegram_id=%s: %s", telegram_id, exc)
        return web.json_response({"error": str(exc)}, status=502)


def create_internal_app(bot: Bot) -> web.Application:
    """Creates an aiohttp application with the internal notification endpoint."""
    app = web.Application()
    app["bot"] = bot
    app.router.add_post("/internal/notify", handle_notify)
    return app
