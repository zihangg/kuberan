from telegram import Update
from telegram.ext import ContextTypes


def handle():
    async def clear_command(update: Update, context: ContextTypes.DEFAULT_TYPE):
        # Send a block of empty lines to push old messages out of view
        await update.message.reply_text("\n" * 50 + "Chat cleared.")

    return clear_command
