"""Due-date reminders (plan 023, 5.6).

The bot has no HTTP server and the API never calls it, so reminders are
pulled: a repeating job asks the API for the reminders that are due, sends each
linked user a message, and confirms every delivery so it is not sent again.
"""
import logging
from datetime import date, datetime
from typing import Any, Dict, Optional

import httpx
from telegram.constants import ParseMode
from telegram.error import Forbidden, TelegramError
from telegram.ext import ContextTypes

from api_client import KuberanAPIClient
from handlers.common import describe_error
from utils.formatting import esc, format_currency

logger = logging.getLogger(__name__)


def format_due_date(due_on: str, today: Optional[date] = None) -> str:
    """Format a YYYY-MM-DD date as "1 Dec" (with the year when it is not this year)."""
    parsed = datetime.strptime(due_on, "%Y-%m-%d").date()
    today = today or date.today()
    text = f"{parsed.day} {parsed.strftime('%b')}"
    return text if parsed.year == today.year else f"{text} {parsed.year}"


def format_fee_date(due_on: str, day_known: bool, today: Optional[date] = None) -> str:
    """Format a fee date: "15 Mar" when the day is known, else "in March" (year if not this year)."""
    if day_known:
        return format_due_date(due_on, today)
    parsed = datetime.strptime(due_on, "%Y-%m-%d").date()
    today = today or date.today()
    text = f"in {parsed.strftime('%B')}"
    return text if parsed.year == today.year else f"{text} {parsed.year}"


def _shortfall(waiver: Dict[str, Any], currency: str) -> str:
    """What is still missing for the waiver: money, transactions, or both."""
    parts = []
    spend = waiver.get("spend")
    if spend and spend.get("remaining", 0) > 0:
        parts.append(f"{format_currency(spend['remaining'], currency)} more")
    swipes = waiver.get("swipes")
    if swipes and swipes.get("remaining", 0) > 0:
        n = swipes["remaining"]
        parts.append(f"{n} more transaction{'' if n == 1 else 's'}")
    return ", or ".join(parts)


def _waiver_deadline(reminder: Dict[str, Any], waiver: Dict[str, Any], today: Optional[date]) -> str:
    """The waiver deadline as a phrase after "before"/"by". Only a card-year window can be month-only."""
    if waiver.get("window") == "card_year":
        known = reminder.get("fee_day_known", True)
        end = waiver.get("window_end") or reminder["due_on"]
        if not known:
            parsed = datetime.strptime(end, "%Y-%m-%d").date()
            today = today or date.today()
            text = parsed.strftime("%B")
            return text if parsed.year == today.year else f"{text} {parsed.year}"
        return format_due_date(end, today)
    return format_due_date(waiver.get("window_end") or reminder["due_on"], today)


def _window_closed(waiver: Dict[str, Any], today: Optional[date]) -> bool:
    """Whether the waiver window has already ended, so its target can no longer be reached."""
    end = waiver.get("window_end")
    if not end:
        return False
    return datetime.strptime(end, "%Y-%m-%d").date() < (today or date.today())


def _missed_by(waiver: Dict[str, Any], currency: str, reminder: Dict[str, Any], today: Optional[date]) -> str:
    """How far short a closed window fell: "RM 3,580.00 short in 2026"."""
    parts = []
    spend = waiver.get("spend")
    if spend and spend.get("remaining", 0) > 0:
        parts.append(format_currency(spend["remaining"], currency))
    swipes = waiver.get("swipes")
    if swipes and swipes.get("remaining", 0) > 0:
        n = swipes["remaining"]
        parts.append(f"{n} transaction{'' if n == 1 else 's'}")
    if waiver.get("window") == "calendar_year":
        when = f"in {waiver['window_end'][:4]}"
    else:
        when = f"by {_waiver_deadline(reminder, waiver, today)}"
    return f"{' or '.join(parts)} short {when}"


def _format_annual_fee(reminder: Dict[str, Any], today: Optional[date]) -> str:
    name = esc(reminder.get("account_name") or "Your account")
    currency = reminder["currency"]
    fee = esc(format_currency(reminder.get("annual_fee") or reminder.get("amount") or 0, currency))
    day_known = reminder.get("fee_day_known", True)
    when = format_fee_date(reminder["due_on"], day_known, today)
    when = esc(when if not day_known else f"on {when}")
    head = f"💳 <b>{name}</b> annual fee of {fee} is due {when}."
    waiver = reminder.get("fee_waiver")
    if not waiver:
        spent = esc(format_currency(reminder.get("spent_last_12_months") or 0, currency))
        return f"{head}\nYou spent {spent} on this card in the last 12 months."
    if waiver.get("status") == "met":
        return f"{head}\nWaiver met — check the fee isn't charged on your statement."
    if _window_closed(waiver, today):
        return (f"{head}\nWaiver target missed ({esc(_missed_by(waiver, currency, reminder, today))}). "
                "Call the bank to ask for a waiver, or cancel.")
    deadline = esc(_waiver_deadline(reminder, waiver, today))
    spend_left = bool((waiver.get("spend") or {}).get("remaining", 0) > 0)
    swipes = waiver.get("swipes") or {}
    swipes_left = swipes.get("remaining", 0)
    if swipes_left > 0 and not spend_left:
        n = swipes_left
        return (f"{head}\nWaiver not met. Make {n} more transaction{'' if n == 1 else 's'} before {deadline}, "
                "call the bank to ask for a waiver, or cancel.")
    short = esc(_shortfall(waiver, currency))
    action = "Spend it or make them" if swipes_left > 0 else "Spend it"
    return (f"{head}\nWaiver not met: {short} before {deadline}. "
            f"{action}, call the bank to ask for a waiver, or cancel.")


def _format_waiver_pace(reminder: Dict[str, Any], today: Optional[date]) -> str:
    name = esc(reminder.get("account_name") or "Your account")
    currency = reminder["currency"]
    fee = esc(format_currency(reminder.get("annual_fee") or reminder.get("amount") or 0, currency))
    waiver = reminder.get("fee_waiver") or {}
    deadline = esc(_waiver_deadline(reminder, waiver, today))
    days = waiver.get("days_left")
    left = f" — {days} day{'' if days == 1 else 's'} left" if days is not None else ""
    short = esc(_shortfall(waiver, currency))
    if not short:
        return f"💳 <b>{name}</b>: waiver of the {fee} annual fee is on track{left}."
    msg = f"💳 <b>{name}</b>: {short} on this card before {deadline} to waive the {fee} annual fee{left}."
    per_month = waiver.get("per_month_needed")
    if per_month:
        msg += f"\nAbout {esc(format_currency(per_month, currency))} a month."
    return msg


def format_reminder_message(reminder: Dict[str, Any], today: Optional[date] = None) -> str:
    """Build the (HTML) message for one due reminder.

    Amounts are in the account's own currency, as the API sends them.
    """
    kind = reminder.get("kind")
    if kind == "card_annual_fee":
        return _format_annual_fee(reminder, today)
    if kind == "fee_waiver_pace":
        return _format_waiver_pace(reminder, today)

    name = esc(reminder.get("account_name") or "Your account")
    when = esc(format_due_date(reminder["due_on"], today))
    currency = reminder["currency"]
    amount = format_currency(reminder.get("amount") or 0, currency)

    if kind == "fd_maturity":
        return f"🏦 <b>{name}</b> matures on {when} — {esc(amount)} at maturity"
    if kind == "loan_due":
        payment = reminder.get("monthly_payment")
        if payment:
            return (f"🏦 <b>{name}</b> payment of {esc(format_currency(payment, currency))} "
                    f"is due on {when} — {esc(amount)} owed")
        return f"🏦 <b>{name}</b> is due on {when} — {esc(amount)} owed"
    return f"💳 <b>{name}</b> is due on {when} — {esc(amount)} owed"


async def send_due_reminders(context: ContextTypes.DEFAULT_TYPE) -> None:
    """Job: send every due reminder and confirm it with the API.

    ``context.job.data`` is the :class:`KuberanAPIClient`. A reminder is
    confirmed only after Telegram accepted the message, so a transient failure
    is retried on the next poll. A user who blocked the bot (Forbidden) can
    never be reached, so that reminder is confirmed too instead of being
    retried forever.
    """
    api: KuberanAPIClient = context.job.data
    try:
        reminders = await api.get_due_reminders()
    except httpx.HTTPError as e:
        logger.warning("Failed to fetch due reminders: %s", describe_error(e))
        return

    for reminder in reminders:
        reminder_id = reminder.get("id")
        chat_id = reminder.get("telegram_user_id")
        if not reminder_id or not chat_id:
            continue
        try:
            await context.bot.send_message(
                chat_id=chat_id,
                text=format_reminder_message(reminder),
                parse_mode=ParseMode.HTML,
            )
        except Forbidden:
            logger.warning("Reminder %s: user blocked the bot; dropping it", reminder_id)
        except TelegramError as e:
            logger.warning("Reminder %s not sent, will retry: %s", reminder_id, describe_error(e))
            continue
        try:
            await api.mark_reminder_sent(reminder_id)
        except httpx.HTTPError as e:
            # Sent but not confirmed: the next poll may deliver it again.
            logger.warning("Reminder %s sent but not confirmed: %s", reminder_id, describe_error(e))
