import asyncio
import os
import sys
from datetime import date
from types import SimpleNamespace

import httpx
import pytest
from telegram.error import Forbidden, NetworkError

sys.path.insert(0, os.path.dirname(os.path.dirname(os.path.abspath(__file__))))

os.environ.setdefault("TELEGRAM_BOT_TOKEN", "123456:TEST")
os.environ.setdefault("BOT_INTERNAL_SECRET", "test")

import main  # noqa: E402
import reminders  # noqa: E402
from api_client import KuberanAPIClient  # noqa: E402

TODAY = date(2026, 11, 9)

CARD = {
    "id": "r1", "kind": "card_due", "telegram_user_id": 777, "account_name": "Amex Gold",
    "due_on": "2026-12-01", "amount": 123450, "currency": "MYR",
}


# --- message formatting ---

def test_card_message():
    assert reminders.format_reminder_message(CARD, TODAY) == "💳 <b>Amex Gold</b> is due on 1 Dec — RM 1,234.50 owed"


def test_message_uses_the_accounts_own_currency():
    msg = reminders.format_reminder_message({**CARD, "currency": "USD", "amount": 50000}, TODAY)
    assert "US$500.00 owed" in msg
    assert "RM" not in msg


def test_loan_message_with_and_without_monthly_payment():
    loan = {**CARD, "kind": "loan_due", "account_name": "Car loan", "amount": 5000000, "due_on": "2026-11-15"}
    assert reminders.format_reminder_message(loan, TODAY) == "🏦 <b>Car loan</b> is due on 15 Nov — RM 50,000.00 owed"
    with_payment = reminders.format_reminder_message({**loan, "monthly_payment": 120000}, TODAY)
    assert with_payment == "🏦 <b>Car loan</b> payment of RM 1,200.00 is due on 15 Nov — RM 50,000.00 owed"


def test_fd_message():
    fd = {**CARD, "kind": "fd_maturity", "account_name": "FD 12M", "amount": 1040000, "due_on": "2027-06-01"}
    assert reminders.format_reminder_message(fd, TODAY) == "🏦 <b>FD 12M</b> matures on 1 Jun 2027 — RM 10,400.00 at maturity"


def test_account_names_are_html_escaped():
    msg = reminders.format_reminder_message({**CARD, "account_name": "A <b>&</b> B"}, TODAY)
    assert "A &lt;b&gt;&amp;&lt;/b&gt; B" in msg


@pytest.mark.parametrize("due_on,expected", [
    ("2026-12-01", "1 Dec"),
    ("2026-11-09", "9 Nov"),
    ("2027-01-31", "31 Jan 2027"),
])
def test_format_due_date(due_on, expected):
    assert reminders.format_due_date(due_on, TODAY) == expected


# --- the polling job ---

class FakeBot:
    def __init__(self, fail=None):
        self.sent = []
        self.fail = fail or {}

    async def send_message(self, chat_id, text, parse_mode=None):
        if chat_id in self.fail:
            raise self.fail[chat_id]
        self.sent.append((chat_id, text, parse_mode))


def _run_job(items, bot, confirm_status=200, due_status=200):
    """Run the job against a mock API; returns the reminder ids confirmed as sent."""
    confirmed = []

    def handler(request: httpx.Request) -> httpx.Response:
        assert request.headers.get("X-Internal-Secret") == "s3cret"
        if request.method == "GET" and request.url.path == "/api/v1/internal/telegram/reminders/due":
            if due_status != 200:
                return httpx.Response(due_status)
            return httpx.Response(200, json={"reminders": items})
        if request.method == "POST" and request.url.path.endswith("/sent"):
            confirmed.append(request.url.path.split("/")[-2])
            return httpx.Response(confirm_status, json={})
        return httpx.Response(404)

    async def run():
        api = KuberanAPIClient("http://api:8080", "s3cret", transport=httpx.MockTransport(handler))
        try:
            ctx = SimpleNamespace(job=SimpleNamespace(data=api), bot=bot)
            await reminders.send_due_reminders(ctx)
        finally:
            await api.aclose()

    asyncio.run(run())
    return confirmed


def test_job_sends_each_reminder_then_confirms_it():
    bot = FakeBot()
    items = [CARD, {**CARD, "id": "r2", "telegram_user_id": 888, "account_name": "Visa"}]
    confirmed = _run_job(items, bot)
    assert [s[0] for s in bot.sent] == [777, 888]
    assert bot.sent[0][2] == "HTML"
    assert "Amex Gold" in bot.sent[0][1]
    assert confirmed == ["r1", "r2"]


def test_job_with_nothing_due_sends_nothing():
    bot = FakeBot()
    assert _run_job([], bot) == []
    assert bot.sent == []


def test_transient_telegram_failure_is_not_confirmed_and_does_not_block_others():
    bot = FakeBot(fail={777: NetworkError("boom")})
    items = [CARD, {**CARD, "id": "r2", "telegram_user_id": 888}]
    confirmed = _run_job(items, bot)
    assert confirmed == ["r2"]
    assert [s[0] for s in bot.sent] == [888]


def test_blocked_user_is_confirmed_so_it_is_not_retried_forever():
    bot = FakeBot(fail={777: Forbidden("bot was blocked by the user")})
    assert _run_job([CARD], bot) == ["r1"]
    assert bot.sent == []


def test_api_error_while_fetching_is_swallowed():
    bot = FakeBot()
    assert _run_job([CARD], bot, due_status=500) == []
    assert bot.sent == []


def test_failed_confirmation_does_not_crash_the_job():
    bot = FakeBot()
    items = [CARD, {**CARD, "id": "r2", "telegram_user_id": 888}]
    confirmed = _run_job(items, bot, confirm_status=500)
    assert confirmed == ["r1", "r2"]  # both attempted
    assert len(bot.sent) == 2


# --- wiring ---

def test_reminder_job_is_registered():
    client = KuberanAPIClient("http://api:8080", "test")
    app = main.build_application(client)
    jobs = app.job_queue.get_jobs_by_name("reminders")
    assert len(jobs) == 1
    assert jobs[0].data is client
    asyncio.run(client.aclose())


# --- annual fee and fee waiver (plan 025) ---

FEE = {
    "id": "f1", "kind": "card_annual_fee", "telegram_user_id": 777, "account_name": "Maybank Visa",
    "due_on": "2027-03-15", "amount": 25000, "currency": "MYR", "annual_fee": 25000,
    "fee_day_known": True, "spent_last_12_months": 842000, "fee_waiver": None,
}
WAIVER = {
    "type": "spend", "window": "card_year", "window_start": "2026-03-16", "window_end": "2027-03-15",
    "spend": {"target": 1200000, "actual": 842000, "remaining": 358000}, "swipes": None,
    "status": "behind", "per_month_needed": 89500, "days_left": 114,
}


def test_annual_fee_message_waiver_met():
    met = {**WAIVER, "status": "met", "per_month_needed": None,
           "spend": {"target": 1200000, "actual": 1300000, "remaining": 0}}
    msg = reminders.format_reminder_message({**FEE, "fee_waiver": met}, TODAY)
    assert msg.startswith("💳 <b>Maybank Visa</b>")
    assert "RM 250.00" in msg and "15 Mar 2027" in msg
    assert "Waiver met — check the fee isn't charged on your statement." in msg


def test_annual_fee_message_waiver_short():
    msg = reminders.format_reminder_message({**FEE, "fee_waiver": WAIVER}, TODAY)
    assert "RM 3,580.00 more" in msg
    assert "before 15 Mar 2027" in msg
    assert "call the bank to ask for a waiver, or cancel" in msg


def test_annual_fee_message_swipes_only_and_both():
    swipes = {"target": 12, "actual": 9, "remaining": 3}
    only = {**WAIVER, "type": "swipes", "spend": None, "swipes": swipes, "per_month_needed": None}
    msg = reminders.format_reminder_message({**FEE, "fee_waiver": only}, TODAY)
    assert "RM 3,580.00" not in msg
    assert ("Make 3 more transactions before 15 Mar 2027, call the bank to ask for a waiver, or cancel."
            in msg)
    assert "Spend it" not in msg
    both = {**WAIVER, "type": "spend_or_swipes", "swipes": swipes}
    msg = reminders.format_reminder_message({**FEE, "fee_waiver": both}, TODAY)
    assert "RM 3,580.00 more, or 3 more transactions" in msg
    assert "Spend it or make them, call the bank to ask for a waiver, or cancel." in msg


def test_annual_fee_message_spend_only_says_spend_it():
    msg = reminders.format_reminder_message({**FEE, "fee_waiver": WAIVER}, TODAY)
    assert "Spend it, call the bank to ask for a waiver, or cancel." in msg


CAL_FEE_TODAY = date(2027, 2, 13)
CAL_WAIVER = {
    "type": "spend", "window": "calendar_year", "window_start": "2026-01-01", "window_end": "2026-12-31",
    "spend": {"target": 1200000, "actual": 842000, "remaining": 358000}, "swipes": None,
    "status": "behind", "per_month_needed": None, "days_left": 0,
}


def test_annual_fee_message_calendar_year_met():
    met = {**CAL_WAIVER, "status": "met", "spend": {"target": 1200000, "actual": 1250000, "remaining": 0}}
    msg = reminders.format_reminder_message({**FEE, "fee_waiver": met}, CAL_FEE_TODAY)
    assert "Waiver met — check the fee isn't charged on your statement." in msg


def test_annual_fee_message_calendar_year_missed():
    msg = reminders.format_reminder_message({**FEE, "fee_waiver": CAL_WAIVER}, CAL_FEE_TODAY)
    assert ("Waiver target missed (RM 3,580.00 short in 2026). "
            "Call the bank to ask for a waiver, or cancel.") in msg
    assert "before" not in msg and "31 Dec" not in msg and "Spend it" not in msg


def test_annual_fee_message_calendar_year_missed_swipes():
    swipes = {**CAL_WAIVER, "type": "swipes", "spend": None, "swipes": {"target": 12, "actual": 9, "remaining": 3}}
    msg = reminders.format_reminder_message({**FEE, "fee_waiver": swipes}, CAL_FEE_TODAY)
    assert "Waiver target missed (3 transactions short in 2026)." in msg


def test_annual_fee_message_calendar_year_still_open():
    # A January fee is reminded about in December, while its deciding year is still open.
    open_window = {**CAL_WAIVER, "days_left": 15, "per_month_needed": 358000}
    msg = reminders.format_reminder_message(
        {**FEE, "due_on": "2027-01-15", "fee_waiver": open_window}, date(2026, 12, 16))
    assert "RM 3,580.00 more before 31 Dec" in msg and "missed" not in msg


def test_annual_fee_message_no_waiver():
    msg = reminders.format_reminder_message(FEE, TODAY)
    assert "RM 250.00" in msg
    assert "You spent RM 8,420.00 on this card in the last 12 months." in msg


def test_annual_fee_message_day_unknown():
    msg = reminders.format_reminder_message({**FEE, "fee_day_known": False}, TODAY)
    assert "in March 2027" in msg and "15" not in msg
    short = reminders.format_reminder_message({**FEE, "fee_day_known": False, "fee_waiver": WAIVER}, TODAY)
    assert "15" not in short and "RM 3,580.00 more" in short


def test_annual_fee_message_uses_card_currency():
    usd_waiver = {**WAIVER, "spend": {"target": 300000, "actual": 100000, "remaining": 200000}}
    msg = reminders.format_reminder_message(
        {**FEE, "currency": "USD", "amount": 9500, "annual_fee": 9500, "fee_waiver": usd_waiver}, TODAY)
    assert "US$95.00" in msg and "US$2,000.00 more" in msg
    assert "RM" not in msg
    no_waiver = reminders.format_reminder_message({**FEE, "currency": "USD", "spent_last_12_months": 500000}, TODAY)
    assert "US$5,000.00" in no_waiver and "RM" not in no_waiver


def test_fee_waiver_pace_message():
    pace = {**FEE, "kind": "fee_waiver_pace", "fee_waiver": WAIVER}
    msg = reminders.format_reminder_message(pace, TODAY)
    assert msg.startswith("💳 <b>Maybank Visa</b>")
    assert "RM 3,580.00 more" in msg
    assert "114 days left" in msg
    assert "RM 895.00 a month" in msg
    assert "15 Mar 2027" in msg


def test_fee_waiver_pace_calendar_year_shows_full_date_even_if_day_unknown():
    cal = {**WAIVER, "window": "calendar_year", "window_end": "2026-12-31"}
    msg = reminders.format_reminder_message(
        {**FEE, "kind": "fee_waiver_pace", "due_on": "2026-12-31", "fee_day_known": False, "fee_waiver": cal}, TODAY)
    assert "31 Dec" in msg and "March" not in msg


def test_fee_waiver_pace_swipes_only_has_no_per_month_line():
    swipes = {**WAIVER, "type": "swipes", "spend": None, "swipes": {"target": 12, "actual": 9, "remaining": 3},
              "per_month_needed": None, "days_left": 1}
    msg = reminders.format_reminder_message({**FEE, "kind": "fee_waiver_pace", "fee_waiver": swipes}, TODAY)
    assert "3 more transactions" in msg and "1 day left" in msg and "a month" not in msg


@pytest.mark.parametrize("due_on,known,expected", [
    ("2026-12-01", True, "1 Dec"),
    ("2027-03-15", True, "15 Mar 2027"),
    ("2026-12-01", False, "in December"),
    ("2028-03-15", False, "in March 2028"),
])
def test_format_fee_date(due_on, known, expected):
    assert reminders.format_fee_date(due_on, known, TODAY) == expected


def test_unknown_kind_falls_back_without_raising():
    msg = reminders.format_reminder_message({**CARD, "kind": "something_new"}, TODAY)
    assert msg == "💳 <b>Amex Gold</b> is due on 1 Dec — RM 1,234.50 owed"
