import logging

from aiogram import Router
from aiogram.filters import Command, CommandStart
from aiogram.types import Message

from ...application.user_use_cases import UserUseCases
from ..keyboards.reply import main_menu_keyboard

logger = logging.getLogger(__name__)

router = Router()


@router.message(CommandStart())
async def start_handler(message: Message, user_use_cases: UserUseCases) -> None:
    tg_user = message.from_user
    if tg_user is None:
        return

    user = await user_use_cases.get_or_register_user(
        telegram_id=tg_user.id,
        username=tg_user.username or "",
        first_name=tg_user.first_name or "",
        last_name=tg_user.last_name or "",
    )

    profile = await user_use_cases.get_profile(user.id)

    if profile is None:
        await message.answer(
            f"👋 Hey, <b>{tg_user.first_name}</b>! Welcome to <b>Dating Bot</b>!\n\n"
            "It looks like you don't have a profile yet.\n"
            "Let's set one up so you can start meeting people! 🚀\n\n"
            "Use /createprofile to get started.",
        )
    else:
        await message.answer(
            f"👋 Welcome back, <b>{tg_user.first_name}</b>!\n\n"
            "What would you like to do today?",
            reply_markup=main_menu_keyboard(),
        )


@router.message(Command("help"))
async def help_handler(message: Message) -> None:
    await message.answer(
        "📖 <b>Available commands</b>\n\n"
        "/start — Start the bot\n"
        "/createprofile — Create your profile\n"
        "/profile — View your profile\n"
        "/edit — Edit your profile\n"
        "/browse — Browse profiles\n"
        "/matches — View your matches\n"
        "/help — Show this message\n\n"
        "You can also use the menu buttons below 👇",
    )
