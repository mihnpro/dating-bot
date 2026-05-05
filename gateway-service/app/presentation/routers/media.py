import logging

from aiogram import F, Router
from aiogram.filters import Command
from aiogram.fsm.context import FSMContext
from aiogram.types import BufferedInputFile, CallbackQuery, Message

from ...application.media_use_cases import MAX_PHOTOS, MediaUseCases
from ...application.user_use_cases import UserUseCases
from ..keyboards.inline import photos_keyboard
from ..keyboards.reply import main_menu_keyboard, remove_keyboard
from ..states.registration import UploadPhoto

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


async def _show_photos(
    message: Message,
    user_id: int,
    media_use_cases: MediaUseCases,
) -> None:
    photos = await media_use_cases.get_user_photos(user_id)
    count = len(photos)

    if not photos:
        await message.answer(
            "📷 <b>Your photos</b>\n\n"
            "You have no photos yet. Profiles with photos get <b>3× more matches</b>!\n\n"
            "Send me a photo to add it to your profile.",
            reply_markup=remove_keyboard,
        )
        return

    # Send all photos as an album so the user sees them
    photo_inputs = []
    for p in photos:
        data = await media_use_cases.get_photo_bytes(p.url)
        if data:
            photo_inputs.append(BufferedInputFile(data, filename=f"photo_{p.id}.jpg"))

    if len(photo_inputs) == 1:
        await message.answer_photo(photo=photo_inputs[0])
    elif len(photo_inputs) > 1:
        from aiogram.types import InputMediaPhoto
        await message.answer_media_group(
            media=[InputMediaPhoto(media=f) for f in photo_inputs]
        )

    await message.answer(
        f"📷 <b>Your photos</b> ({count}/{MAX_PHOTOS})\n\n"
        f"{'✅ Profile looks great!' if count >= 3 else '💡 Tip: add at least 3 photos for the best results.'}\n\n"
        "Tap a photo number to view it, or delete it.\n"
        "To add more — just send a photo here.",
        reply_markup=photos_keyboard(photos),
    )


# =========================================================================== #
# Entry point — /photos command and menu button                                 #
# =========================================================================== #


@router.message(Command("photos"))
@router.message(F.text == "📷 My Photos")
async def photos_handler(
    message: Message,
    state: FSMContext,
    user_use_cases: UserUseCases,
    media_use_cases: MediaUseCases,
) -> None:
    user = await _get_current_user(message, user_use_cases)
    await state.set_state(UploadPhoto.waiting_for_photo)
    await _show_photos(message, user.id, media_use_cases)


# =========================================================================== #
# Receive photo from user                                                       #
# =========================================================================== #


@router.message(UploadPhoto.waiting_for_photo, F.photo)
async def receive_photo(
    message: Message,
    state: FSMContext,
    user_use_cases: UserUseCases,
    media_use_cases: MediaUseCases,
) -> None:
    user = await _get_current_user(message, user_use_cases)

    # Telegram sends multiple sizes — take the largest (last in list)
    photo = message.photo[-1]

    wait_msg = await message.answer("⏳ Uploading your photo...")

    media = await media_use_cases.upload_from_telegram(
        user_id=user.id,
        file_id=photo.file_id,
        mime_type="image/jpeg",
    )

    await wait_msg.delete()

    if media is None:
        photos = await media_use_cases.get_user_photos(user.id)
        if len(photos) >= MAX_PHOTOS:
            await message.answer(
                f"⚠️ You've reached the limit of <b>{MAX_PHOTOS} photos</b>.\n"
                "Delete an existing photo to upload a new one.",
                reply_markup=photos_keyboard(photos),
            )
        else:
            await message.answer("❌ Failed to upload the photo. Please try again.")
        return

    await message.answer(
        "✅ Photo uploaded!" + (" It's now your cover photo." if media.is_main else ""),
    )
    await _show_photos(message, user.id, media_use_cases)


@router.message(UploadPhoto.waiting_for_photo, ~F.photo)
async def receive_non_photo(message: Message) -> None:
    if message.text in ("📷 My Photos", "/photos"):
        return  # handled by the command handler above
    await message.answer("📷 Please send a <b>photo</b> (not a file or sticker).")


# =========================================================================== #
# Inline callbacks: add / delete / set main                                     #
# =========================================================================== #


@router.callback_query(F.data == "photo_add")
async def cb_photo_add(
    call: CallbackQuery,
    state: FSMContext,
    user_use_cases: UserUseCases,
    media_use_cases: MediaUseCases,
) -> None:
    await call.answer()
    user = await user_use_cases.get_or_register_user(
        telegram_id=call.from_user.id,
        username=call.from_user.username or "",
        first_name=call.from_user.first_name or "",
        last_name=call.from_user.last_name or "",
    )
    photos = await media_use_cases.get_user_photos(user.id)
    if len(photos) >= MAX_PHOTOS:
        await call.message.answer(
            f"⚠️ You've reached the limit of <b>{MAX_PHOTOS} photos</b>.\n"
            "Delete an existing one first."
        )
        return

    await state.set_state(UploadPhoto.waiting_for_photo)
    await call.message.answer("📷 Send me a photo to add it to your profile.")


@router.callback_query(F.data.startswith("photo_del:"))
async def cb_photo_delete(
    call: CallbackQuery,
    user_use_cases: UserUseCases,
    media_use_cases: MediaUseCases,
) -> None:
    media_id = int(call.data.split(":")[1])
    user = await user_use_cases.get_or_register_user(
        telegram_id=call.from_user.id,
        username=call.from_user.username or "",
        first_name=call.from_user.first_name or "",
        last_name=call.from_user.last_name or "",
    )

    success = await media_use_cases.delete_photo(media_id, user.id)
    if success:
        await call.answer("🗑 Photo deleted")
        photos = await media_use_cases.get_user_photos(user.id)
        if photos:
            await call.message.edit_reply_markup(reply_markup=photos_keyboard(photos))
        else:
            await call.message.edit_text(
                "📷 <b>Your photos</b>\n\nYou have no photos yet. Send me a photo to add one."
            )
    else:
        await call.answer("❌ Could not delete the photo. Try again.", show_alert=True)


@router.callback_query(F.data.startswith("photo_view:"))
async def cb_photo_view(
    call: CallbackQuery,
    user_use_cases: UserUseCases,
    media_use_cases: MediaUseCases,
) -> None:
    media_id = int(call.data.split(":")[1])
    user = await user_use_cases.get_or_register_user(
        telegram_id=call.from_user.id,
        username=call.from_user.username or "",
        first_name=call.from_user.first_name or "",
        last_name=call.from_user.last_name or "",
    )

    photos = await media_use_cases.get_user_photos(user.id)
    photo = next((p for p in photos if p.id == media_id), None)

    if photo is None:
        await call.answer("Photo not found.", show_alert=True)
        return

    data = await media_use_cases.get_photo_bytes(photo.url)
    if data is None:
        await call.answer("Could not load photo.", show_alert=True)
        return

    await call.answer()
    await call.message.answer_photo(
        photo=BufferedInputFile(data, filename=f"photo_{media_id}.jpg")
    )


# =========================================================================== #
# Exit photo management — go back to main menu                                  #
# =========================================================================== #


@router.message(UploadPhoto.waiting_for_photo, F.text == "🏠 Menu")
async def exit_photos(message: Message, state: FSMContext) -> None:
    await state.clear()
    await message.answer("Back to main menu.", reply_markup=main_menu_keyboard())
