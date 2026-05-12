"""Aiogram middleware that tracks Telegram update and FSM session metrics."""

from typing import Any, Awaitable, Callable

from aiogram import BaseMiddleware
from aiogram.fsm.context import FSMContext
from aiogram.types import CallbackQuery, Message, TelegramObject, Update

from ...infrastructure.metrics import (
    active_fsm_sessions,
    callback_actions_total,
    commands_total,
    telegram_updates_total,
)


class MetricsMiddleware(BaseMiddleware):
    """Outer middleware applied to every incoming Update.

    Counts:
    - updates by type (message, callback_query, …)
    - commands (/start, /browse, …)
    - callback action prefix (like, pass, view_match, …)
    - active FSM sessions (gauge incremented on FSM state set, decremented on clear)
    """

    async def __call__(
        self,
        handler: Callable[[TelegramObject, dict[str, Any]], Awaitable[Any]],
        event: TelegramObject,
        data: dict[str, Any],
    ) -> Any:
        if isinstance(event, Update):
            update_type = self._update_type(event)
            telegram_updates_total.labels(update_type).inc()

            if event.message:
                self._track_command(event.message)

            if event.callback_query:
                self._track_callback(event.callback_query)

        result = await handler(event, data)

        # Track FSM session count after the handler runs so state changes are visible.
        state: FSMContext | None = data.get("state")
        if state is not None:
            current = await state.get_state()
            active_fsm_sessions.set(1 if current is not None else 0)

        return result

    @staticmethod
    def _update_type(update: Update) -> str:
        if update.message:
            return "message"
        if update.callback_query:
            return "callback_query"
        if update.inline_query:
            return "inline_query"
        if update.edited_message:
            return "edited_message"
        return "other"

    @staticmethod
    def _track_command(message: Message) -> None:
        text = message.text or ""
        if text.startswith("/"):
            # Strip the "/", take everything before the first space or @.
            command = text[1:].split()[0].split("@")[0].lower()
            commands_total.labels(command).inc()

    @staticmethod
    def _track_callback(cb: CallbackQuery) -> None:
        data = cb.data or ""
        # Callback data format: "action:payload" or just "action".
        action = data.split(":")[0].lower() if data else "unknown"
        callback_actions_total.labels(action).inc()
