import asyncio
import logging

from aiogram import F, Router
from aiogram.filters import Command
from aiogram.fsm.context import FSMContext
from aiogram.types import BufferedInputFile, InputMediaPhoto, Message

from ...application.media_use_cases import MediaUseCases
from ...application.user_use_cases import UserUseCases
from ..keyboards.reply import (
    edit_field_keyboard,
    gender_keyboard,
    main_menu_keyboard,
    remove_keyboard,
)
from ..states.registration import EditProfile, Registration

logger = logging.getLogger(__name__)

router = Router()


# =========================================================================== #
# Helpers                                                                       #
# =========================================================================== #


async def _get_current_user(message: Message, user_use_cases: UserUseCases):
    tg = message.from_user
    return await user_use_cases.get_or_register_user(
        telegram_id=tg.id,
        username=tg.username or "",
        first_name=tg.first_name or "",
        last_name=tg.last_name or "",
    )


async def _get_all_photo_bytes(
    user_id: int, media_use_cases: MediaUseCases
) -> list[bytes]:
    try:
        photos = await media_use_cases.get_user_photos(user_id)
        results = await asyncio.gather(
            *(media_use_cases.get_photo_bytes(p.url) for p in photos),
            return_exceptions=True,
        )
        return [r for r in results if isinstance(r, bytes) and r]
    except Exception:
        return []


async def _send_profile_card(
    message: Message,
    text: str,
    all_photo_bytes: list[bytes],
    reply_markup=None,
) -> None:
    if len(all_photo_bytes) == 1:
        await message.answer_photo(
            photo=BufferedInputFile(all_photo_bytes[0], filename="photo.jpg"),
            caption=text,
            parse_mode="HTML",
            reply_markup=reply_markup,
        )
    elif len(all_photo_bytes) > 1:
        media = [
            InputMediaPhoto(media=BufferedInputFile(data, filename=f"photo_{i}.jpg"))
            for i, data in enumerate(all_photo_bytes)
        ]
        await message.answer_media_group(media=media)
        await message.answer(text, parse_mode="HTML", reply_markup=reply_markup)
    else:
        await message.answer(text, reply_markup=reply_markup)


# =========================================================================== #
# View profile                                                                  #
# =========================================================================== #


@router.message(Command("profile"))
@router.message(F.text == "👤 My Profile")
async def view_profile_handler(
    message: Message,
    user_use_cases: UserUseCases,
    media_use_cases: MediaUseCases,
) -> None:
    user = await _get_current_user(message, user_use_cases)
    profile = await user_use_cases.get_profile(user.id)

    if profile is None:
        await message.answer(
            "You don't have a profile yet!\n\n"
            "Use /createprofile to set one up — it only takes a minute. 🚀"
        )
        return

    card = profile.format_card(name=message.from_user.first_name)
    photo_bytes = await _get_all_photo_bytes(user.id, media_use_cases)
    await _send_profile_card(
        message,
        f"👤 <b>Your Profile</b>\n\n{card}\n\nUse /edit to update your profile.",
        photo_bytes,
        reply_markup=main_menu_keyboard(),
    )


# =========================================================================== #
# Registration FSM                                                              #
# =========================================================================== #


@router.message(Command("createprofile"))
async def start_registration(
    message: Message,
    state: FSMContext,
    user_use_cases: UserUseCases,
    media_use_cases: MediaUseCases,
) -> None:
    user = await _get_current_user(message, user_use_cases)
    existing = await user_use_cases.get_profile(user.id)
    if existing:
        card = existing.format_card(name=message.from_user.first_name)
        photo_bytes = await _get_all_photo_bytes(user.id, media_use_cases)
        await _send_profile_card(
            message,
            f"You already have a profile!\n\n{card}\n\nUse /edit to change it.",
            photo_bytes,
            reply_markup=main_menu_keyboard(),
        )
        return

    await message.answer(
        "Let's create your profile! 🚀\n\n"
        "🎂 <b>Step 1/4</b> — How old are you? (Enter a number, e.g. <code>25</code>)",
        reply_markup=remove_keyboard,
    )
    await state.set_state(Registration.age)


@router.message(Registration.age)
async def reg_age_handler(message: Message, state: FSMContext) -> None:
    text = (message.text or "").strip()
    if not text.isdigit() or not (16 <= int(text) <= 80):
        await message.answer(
            "⚠️ Please enter a valid age between <b>16</b> and <b>80</b>:"
        )
        return

    await state.update_data(age=int(text))
    await message.answer(
        "⚧ <b>Step 2/4</b> — What is your gender?",
        reply_markup=gender_keyboard(),
    )
    await state.set_state(Registration.gender)


@router.message(Registration.gender)
async def reg_gender_handler(message: Message, state: FSMContext) -> None:
    text = (message.text or "").strip()
    if text == "👨 Male":
        gender = "male"
    elif text == "👩 Female":
        gender = "female"
    else:
        await message.answer(
            "Please select your gender using the buttons below:",
            reply_markup=gender_keyboard(),
        )
        return

    await state.update_data(gender=gender)
    await message.answer(
        "🏙 <b>Step 3/4</b> — What city are you from? (e.g. <code>Moscow</code>)",
        reply_markup=remove_keyboard,
    )
    await state.set_state(Registration.city)


@router.message(Registration.city)
async def reg_city_handler(message: Message, state: FSMContext) -> None:
    city = (message.text or "").strip()
    if len(city) < 2:
        await message.answer(
            "⚠️ Please enter a valid city name (at least 2 characters):"
        )
        return

    await state.update_data(city=city)
    await message.answer(
        "🎯 <b>Step 4/4</b> — What are your interests?\n"
        "Enter them separated by commas, e.g. <code>music, travel, sports</code>"
    )
    await state.set_state(Registration.interests)


@router.message(Registration.interests)
async def reg_interests_handler(
    message: Message,
    state: FSMContext,
    user_use_cases: UserUseCases,
    media_use_cases: MediaUseCases,
) -> None:
    text = (message.text or "").strip()
    interests = [i.strip() for i in text.split(",") if i.strip()]
    if not interests:
        await message.answer("⚠️ Please enter at least one interest:")
        return

    data = await state.get_data()
    user = await _get_current_user(message, user_use_cases)

    try:
        profile = await user_use_cases.create_profile(
            user_id=user.id,
            age=data["age"],
            gender=data["gender"],
            city=data["city"],
            interests=interests,
        )
    except Exception as exc:
        logger.error("Failed to create profile for user_id=%s: %s", user.id, exc)
        await message.answer(
            "❌ Something went wrong while saving your profile. Please try again."
        )
        await state.clear()
        return

    await state.clear()
    user = await _get_current_user(message, user_use_cases)
    card = profile.format_card(name=message.from_user.first_name)
    photo_bytes = await _get_all_photo_bytes(user.id, media_use_cases)
    await _send_profile_card(
        message,
        f"✅ <b>Profile created successfully!</b>\n\n{card}",
        photo_bytes,
        reply_markup=main_menu_keyboard(),
    )


# =========================================================================== #
# Edit Profile FSM                                                              #
# =========================================================================== #


@router.message(Command("edit"))
async def edit_profile_start(message: Message, state: FSMContext) -> None:
    await message.answer(
        "✏️ What would you like to update?",
        reply_markup=edit_field_keyboard(),
    )
    await state.set_state(EditProfile.choose_field)


@router.message(EditProfile.choose_field)
async def edit_choose_field(message: Message, state: FSMContext) -> None:
    text = (message.text or "").strip()

    if text == "❌ Cancel":
        await state.clear()
        await message.answer("Cancelled.", reply_markup=main_menu_keyboard())
        return

    if text == "🎂 Age":
        await message.answer("🎂 Enter your new age:", reply_markup=remove_keyboard)
        await state.set_state(EditProfile.age)
    elif text == "🏙 City":
        await message.answer("🏙 Enter your new city:", reply_markup=remove_keyboard)
        await state.set_state(EditProfile.city)
    elif text == "🎯 Interests":
        await message.answer(
            "🎯 Enter your new interests (comma-separated):",
            reply_markup=remove_keyboard,
        )
        await state.set_state(EditProfile.interests)
    else:
        await message.answer(
            "Please choose a field using the buttons below:",
            reply_markup=edit_field_keyboard(),
        )


@router.message(EditProfile.age)
async def edit_age_handler(
    message: Message,
    state: FSMContext,
    user_use_cases: UserUseCases,
) -> None:
    text = (message.text or "").strip()
    if not text.isdigit() or not (16 <= int(text) <= 80):
        await message.answer(
            "⚠️ Please enter a valid age between <b>16</b> and <b>80</b>:"
        )
        return

    user = await _get_current_user(message, user_use_cases)
    await user_use_cases.update_profile(user.id, age=int(text))
    await state.clear()
    await message.answer(
        "✅ Age updated successfully!", reply_markup=main_menu_keyboard()
    )


@router.message(EditProfile.city)
async def edit_city_handler(
    message: Message,
    state: FSMContext,
    user_use_cases: UserUseCases,
) -> None:
    city = (message.text or "").strip()
    if len(city) < 2:
        await message.answer("⚠️ Please enter a valid city name:")
        return

    user = await _get_current_user(message, user_use_cases)
    await user_use_cases.update_profile(user.id, city=city)
    await state.clear()
    await message.answer(
        "✅ City updated successfully!", reply_markup=main_menu_keyboard()
    )


@router.message(EditProfile.interests)
async def edit_interests_handler(
    message: Message,
    state: FSMContext,
    user_use_cases: UserUseCases,
) -> None:
    text = (message.text or "").strip()
    interests = [i.strip() for i in text.split(",") if i.strip()]
    if not interests:
        await message.answer("⚠️ Please enter at least one interest:")
        return

    user = await _get_current_user(message, user_use_cases)
    await user_use_cases.update_profile(user.id, interests=interests)
    await state.clear()
    await message.answer(
        "✅ Interests updated successfully!", reply_markup=main_menu_keyboard()
    )
