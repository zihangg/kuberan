package services

import (
	"time"

	"kuberan/internal/models"
)

// feeDateIn is the fee date of a given year: day of month (nil means the 1st),
// clamped to the month's last day, so 29 Feb becomes 28 Feb in non-leap years.
func feeDateIn(year, month int, day *int) time.Time {
	d := 1
	if day != nil {
		d = *day
	}
	last := time.Date(year, time.Month(month)+1, 0, 0, 0, 0, 0, time.UTC).Day()
	if d > last {
		d = last
	}
	return time.Date(year, time.Month(month), d, 0, 0, 0, 0, time.UTC)
}

// NextFeeDate returns the first date on or after from (compared by UTC date)
// that falls on month/day. A nil day means the 1st. A day the month does not
// have is clamped to its last day.
func NextFeeDate(month int, day *int, from time.Time) time.Time {
	from = utcDay(from)
	candidate := feeDateIn(from.Year(), month, day)
	if candidate.Before(from) {
		candidate = feeDateIn(from.Year()+1, month, day)
	}
	return candidate
}

// FeeWaiverWindow returns the inclusive UTC dates of the waiver window that
// contains from or, for a card-year window, the one ending at the next fee
// date. card_year runs from the day after the previous fee date to the next fee
// date; calendar_year runs from 1 January to 31 December of from's year.
func FeeWaiverWindow(fee *models.CardAnnualFee, from time.Time) (start, end time.Time) {
	from = utcDay(from)
	if fee.WaiverWindow == models.WaiverWindowCalendarYear {
		return time.Date(from.Year(), time.January, 1, 0, 0, 0, 0, time.UTC),
			time.Date(from.Year(), time.December, 31, 0, 0, 0, 0, time.UTC)
	}
	end = NextFeeDate(fee.FeeMonth, fee.FeeDay, from)
	start = feeDateIn(end.Year()-1, fee.FeeMonth, fee.FeeDay).AddDate(0, 0, 1)
	return start, end
}

// FeeDecidingWindow returns the inclusive UTC dates of the waiver window that
// decides the fee charged on feeDate. A card-year window ends on the fee date.
// Spend in calendar year Y waives the fee charged in year Y+1, so a
// calendar-year fee is decided by the calendar year before it, except a fee on
// 31 December, which is decided by its own year.
func FeeDecidingWindow(fee *models.CardAnnualFee, feeDate time.Time) (start, end time.Time) {
	feeDate = utcDay(feeDate)
	if fee.WaiverWindow != models.WaiverWindowCalendarYear {
		return FeeWaiverWindow(fee, feeDate)
	}
	year := feeDate.Year() - 1
	if feeDate.Month() == time.December && feeDate.Day() == 31 {
		year = feeDate.Year()
	}
	return time.Date(year, time.January, 1, 0, 0, 0, 0, time.UTC),
		time.Date(year, time.December, 31, 0, 0, 0, 0, time.UTC)
}
