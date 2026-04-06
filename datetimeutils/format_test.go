package datetimeutils

import (
	"testing"
	"time"
)

func TestFormatEpochSeconds(t *testing.T) {
	// 2026-04-02T00:00:00Z = 1774915200 seconds = 1774915200000 ms
	ms := int64(1774915200000)
	got := FormatEpochSeconds(ms)
	want := "1774915200.000"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestFormatEpochMillis(t *testing.T) {
	ms := int64(1774915200000)
	got := FormatEpochMillis(ms)
	want := "1774915200000"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestFormatEpochMicros(t *testing.T) {
	ms := int64(1774915200000)
	got := FormatEpochMicros(ms)
	want := "1774915200000000"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestFormatEpochNanos(t *testing.T) {
	ms := int64(1774915200000)
	got := FormatEpochNanos(ms)
	want := "1774915200000000000"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestFormatRFC3339(t *testing.T) {
	ms := FromTime(time.Date(2026, 4, 2, 12, 30, 0, 0, time.UTC))
	got := FormatRFC3339(ms)
	want := "2026-04-02T12:30:00Z"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestFormatRFC3339Nano(t *testing.T) {
	ms := FromTime(time.Date(2026, 4, 2, 12, 30, 0, 0, time.UTC))
	got := FormatRFC3339Nano(ms)
	// millisecond precision from epoch ms
	want := "2026-04-02T12:30:00Z"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestFormatDate(t *testing.T) {
	ms := FromTime(time.Date(2026, 4, 2, 12, 30, 0, 0, time.UTC))
	got := FormatDate(ms)
	want := "2026-04-02"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestFormatDateTime(t *testing.T) {
	ms := FromTime(time.Date(2026, 4, 2, 12, 30, 45, 0, time.UTC))
	got := FormatDateTime(ms)
	want := "2026-04-02 12:30:45"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestFormatCustom(t *testing.T) {
	ms := FromTime(time.Date(2026, 4, 2, 12, 30, 0, 0, time.UTC))
	got := FormatCustom(ms, "02/01/2006")
	want := "02/04/2026"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestFormatInTimezone(t *testing.T) {
	ms := FromTime(time.Date(2026, 4, 2, 0, 0, 0, 0, time.UTC))
	got, err := FormatInTimezone(ms, "Asia/Kolkata")
	if err != nil {
		t.Fatal(err)
	}
	want := "2026-04-02T05:30:00+05:30"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestFormatInTimezone_InvalidTZ(t *testing.T) {
	_, err := FormatInTimezone(0, "INVALID/TZ")
	if err == nil {
		t.Error("expected error for invalid timezone")
	}
}

func TestFormatLookbackMillis(t *testing.T) {
	start := FromTime(time.Date(2026, 4, 2, 11, 0, 0, 0, time.UTC))
	end := FromTime(time.Date(2026, 4, 2, 12, 0, 0, 0, time.UTC))
	got := FormatLookbackMillis(start, end)
	want := "3600000" // 1 hour in ms
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestToTime(t *testing.T) {
	want := time.Date(2026, 4, 2, 12, 0, 0, 0, time.UTC)
	ms := FromTime(want)
	got := ToTime(ms)
	if !got.Equal(want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestFromTime(t *testing.T) {
	src := time.Date(2026, 4, 2, 12, 0, 0, 0, time.UTC)
	got := FromTime(src)
	back := ToTime(got)
	if !back.Equal(src) {
		t.Errorf("roundtrip failed: got %v, want %v", back, src)
	}
}
