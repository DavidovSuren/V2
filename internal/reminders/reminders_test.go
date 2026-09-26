package reminders

import (
	"testing"
	"time"
)

func TestOptions(t *testing.T) {
	o := Options()
	if o[0] != "07:00" || o[len(o)-1] != "22:00" {
		t.Fatalf("границы: %s … %s", o[0], o[len(o)-1])
	}
	if len(o) != 31+1 { // 07:00..22:00 через 30 минут = 31, плюс 15:15
		t.Errorf("вариантов %d", len(o))
	}
	for i := 1; i < len(o); i++ {
		if o[i] <= o[i-1] {
			t.Errorf("порядок: %s после %s", o[i], o[i-1])
		}
	}
	for _, ok := range []string{"07:00", "07:30", "15:15", "21:30", "22:00"} {
		if !Valid(ok) {
			t.Errorf("%s должно быть допустимо", ok)
		}
	}
	for _, bad := range []string{"", "06:30", "22:30", "07:15", "7:00", "ночью"} {
		if Valid(bad) {
			t.Errorf("%q не должно быть допустимо", bad)
		}
	}
}

func TestDue(t *testing.T) {
	msk := time.FixedZone("MSK", 3*3600)
	at := func(h, m int) string { return Clock(time.Date(2026, 10, 1, h, m, 20, 0, msk), msk) }
	if !Due("", at(15, 15)) || Due("", at(15, 16)) {
		t.Error("время по умолчанию 15:15")
	}
	if !Due("09:00", at(9, 0)) || Due("09:00", at(15, 15)) {
		t.Error("выбранное время")
	}
	if !Due("бред", at(15, 15)) {
		t.Error("некорректное время — по умолчанию")
	}
	// 06:00 UTC = 09:00 МСК.
	if Clock(time.Date(2026, 10, 1, 6, 0, 0, 0, time.UTC), msk) != "09:00" {
		t.Error("часовой пояс")
	}
	if !EveningDue(3, "21:00") || EveningDue(2, "21:00") || EveningDue(10, "20:30") {
		t.Error("вечернее напоминание")
	}
}

func TestDaysWord(t *testing.T) {
	cases := map[int]string{1: "1 день", 2: "2 дня", 4: "4 дня", 5: "5 дней", 11: "11 дней", 12: "12 дней", 21: "21 день", 22: "22 дня", 111: "111 дней"}
	for n, want := range cases {
		if got := DaysWord(n); got != want {
			t.Errorf("DaysWord(%d) = %q, want %q", n, got, want)
		}
	}
}
