package services

import (
	"testing"
	"time"

	"kuberan/internal/models"
)

func TestNextFeeDate(t *testing.T) {
	d := func(v int) *int { return &v }
	tests := []struct {
		name  string
		month int
		day   *int
		from  time.Time
		want  time.Time
	}{
		{"later this cycle", 3, d(15), utcYMD(2026, 11, 21), utcYMD(2027, 3, 15)},
		{"no day uses the 1st", 3, nil, utcYMD(2026, 11, 21), utcYMD(2027, 3, 1)},
		{"the date itself", 11, d(21), utcYMD(2026, 11, 21), utcYMD(2026, 11, 21)},
		{"29 Feb clamps in a non-leap year", 2, d(29), utcYMD(2026, 11, 21), utcYMD(2027, 2, 28)},
		{"29 Feb in a leap year", 2, d(29), utcYMD(2027, 3, 1), utcYMD(2028, 2, 29)},
		{"31 Apr clamps to 30", 4, d(31), utcYMD(2026, 1, 1), utcYMD(2026, 4, 30)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NextFeeDate(tt.month, tt.day, tt.from)
			if !got.Equal(tt.want) {
				t.Fatalf("NextFeeDate(%d, %v, %v) = %v, want %v", tt.month, tt.day, tt.from, got, tt.want)
			}
		})
	}
}

func TestFeeWaiverWindow(t *testing.T) {
	d15 := 15
	tests := []struct {
		name       string
		window     models.WaiverWindow
		from       time.Time
		start, end time.Time
	}{
		{"card year", models.WaiverWindowCardYear, utcYMD(2026, 11, 21), utcYMD(2026, 3, 16), utcYMD(2027, 3, 15)},
		{"card year the day after the fee", models.WaiverWindowCardYear, utcYMD(2027, 3, 16), utcYMD(2027, 3, 16), utcYMD(2028, 3, 15)},
		{"calendar year", models.WaiverWindowCalendarYear, utcYMD(2026, 11, 21), utcYMD(2026, 1, 1), utcYMD(2026, 12, 31)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fee := &models.CardAnnualFee{FeeMonth: 3, FeeDay: &d15, WaiverWindow: tt.window}
			start, end := FeeWaiverWindow(fee, tt.from)
			if !start.Equal(tt.start) || !end.Equal(tt.end) {
				t.Fatalf("FeeWaiverWindow(%s, %v) = [%v, %v], want [%v, %v]", tt.window, tt.from, start, end, tt.start, tt.end)
			}
		})
	}
}

func TestFeeWaiverWindowLeapFeeDate(t *testing.T) {
	// A 29 Feb fee: the previous fee date was 28 Feb 2027, so the window starts 1 Mar.
	d29 := 29
	fee := &models.CardAnnualFee{FeeMonth: 2, FeeDay: &d29, WaiverWindow: models.WaiverWindowCardYear}
	start, end := FeeWaiverWindow(fee, utcYMD(2027, 6, 1))
	if want := utcYMD(2027, 3, 1); !start.Equal(want) {
		t.Fatalf("start = %v, want %v", start, want)
	}
	if want := utcYMD(2028, 2, 29); !end.Equal(want) {
		t.Fatalf("end = %v, want %v", end, want)
	}
}
