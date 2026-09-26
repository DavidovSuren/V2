package payouts

import (
	"testing"
	"time"
)

func TestSplit(t *testing.T) {
	b := Split(1000, 13)
	if b.Tax != 130 || b.Net != 870 || b.Gross != 1000 {
		t.Errorf("%+v", b)
	}
	if b := Split(555, 13); b.Tax+b.Net != 555 {
		t.Errorf("сумма не сходится: %+v", b)
	}
}

func TestPayoutDates(t *testing.T) {
	msk := time.FixedZone("MSK", 3*3600)
	d14 := time.Date(2026, 10, 14, 12, 0, 0, 0, msk)
	d15 := time.Date(2026, 10, 15, 0, 30, 0, 0, msk)
	d16 := time.Date(2026, 10, 16, 12, 0, 0, 0, msk)
	if IsPayoutDay(d14, msk) || !IsPayoutDay(d15, msk) || IsPayoutDay(d16, msk) {
		t.Error("IsPayoutDay")
	}
	// 14 октября 22:00 UTC = 15 октября 01:00 МСК.
	if !IsPayoutDay(time.Date(2026, 10, 14, 22, 0, 0, 0, time.UTC), msk) {
		t.Error("часовой пояс не учтён")
	}
	if got := HumanDate(NextPayoutDate(d16, msk)); got != "15 ноября" {
		t.Errorf("следующая дата: %s", got)
	}
	if got := HumanDate(NextPayoutDate(d14, msk)); got != "15 октября" {
		t.Errorf("следующая дата: %s", got)
	}
	if MonthKey(d15, msk) != "2026-10" {
		t.Error("MonthKey")
	}
}

func TestCard(t *testing.T) {
	if !ValidCard("4111111111111111") || ValidCard("4111111111111112") || ValidCard("411111") {
		t.Error("ValidCard")
	}
	if NormalizeCard("4111 1111-1111 1111") != "4111111111111111" {
		t.Error("NormalizeCard")
	}
	if MaskCard("2200123412341234") != "2200 •••• •••• 1234" {
		t.Error("MaskCard")
	}
}

func TestINN(t *testing.T) {
	if !ValidINN("500100732259") || ValidINN("500100732258") || ValidINN("12345") {
		t.Error("ValidINN")
	}
}
