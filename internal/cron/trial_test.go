package cron

import (
	"database/sql"
	"testing"
	"time"
)

func TestTrialEndsTomorrow(t *testing.T) {
	now := time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC)
	reg := func(ago time.Duration) string { return now.Add(-ago).Format(time.RFC3339) }
	active := sql.NullString{String: now.Add(24 * time.Hour).Format(time.RFC3339), Valid: true}

	cases := []struct {
		name    string
		created string
		tier    string
		expires sql.NullString
		want    bool
	}{
		{"зарегистрирован сегодня", reg(time.Hour), "free", sql.NullString{}, false},
		{"вчера", reg(26 * time.Hour), "free", sql.NullString{}, false},
		{"2 дня назад — завтра конец", reg(50 * time.Hour), "free", sql.NullString{}, true},
		{"ровно 2 дня назад", reg(48 * time.Hour), "free", sql.NullString{}, true},
		{"пробный уже закончился", reg(73 * time.Hour), "free", sql.NullString{}, false},
		{"с подпиской не пишем", reg(50 * time.Hour), "plus369", active, false},
	}
	for _, c := range cases {
		if got := trialEndsTomorrow(c.created, c.tier, c.expires, now); got != c.want {
			t.Errorf("%s: %v, want %v", c.name, got, c.want)
		}
	}
}

func TestPaydayText(t *testing.T) {
	if got := paydayText(5772); got[:len("Сегодня день вывода: доступно 5 772 ₽")] != "Сегодня день вывода: доступно 5 772 ₽" {
		t.Errorf("текст: %q", got)
	}
}
