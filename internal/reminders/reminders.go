// Package reminders — правила ежедневных напоминаний бота (этап 11): время
// выбирает пользователь (07:00–22:00, шаг 30 минут, по Москве), в 21:00 —
// мягкое второе напоминание тем, чей стрик может сгореть.
package reminders

import (
	"fmt"
	"time"
)

// DefaultAt — время по умолчанию (как было в Node-версии).
const DefaultAt = "15:15"

// EveningAt — второе напоминание тем, у кого стрик ≥ EveningMinStreak.
const EveningAt = "21:00"

const EveningMinStreak = 3

// Options — варианты времени: 07:00, 07:30 … 22:00 и время по умолчанию.
func Options() []string {
	var out []string
	for m := 7 * 60; m <= 22*60; m += 30 {
		hhmm := fmt.Sprintf("%02d:%02d", m/60, m%60)
		if hhmm > DefaultAt && (len(out) == 0 || out[len(out)-1] < DefaultAt) {
			out = append(out, DefaultAt)
		}
		out = append(out, hhmm)
	}
	return out
}

// Valid — время из списка Options.
func Valid(hhmm string) bool {
	for _, o := range Options() {
		if o == hhmm {
			return true
		}
	}
	return false
}

// Effective — выбранное время или время по умолчанию.
func Effective(remindAt string) string {
	if Valid(remindAt) {
		return remindAt
	}
	return DefaultAt
}

// Clock — "HH:MM" по Москве для момента now.
func Clock(now time.Time, msk *time.Location) string {
	return now.In(msk).Format("15:04")
}

// Due — пора слать ежедневное напоминание пользователю с временем remindAt.
func Due(remindAt, clock string) bool {
	return Effective(remindAt) == clock
}

// EveningDue — пора слать вечернее «стрик сгорит в полночь».
func EveningDue(streak int, clock string) bool {
	return clock == EveningAt && streak >= EveningMinStreak
}

// DaysWord — «1 день», «3 дня», «12 дней».
func DaysWord(n int) string {
	mod10, mod100 := n%10, n%100
	switch {
	case mod10 == 1 && mod100 != 11:
		return fmt.Sprintf("%d день", n)
	case mod10 >= 2 && mod10 <= 4 && (mod100 < 12 || mod100 > 14):
		return fmt.Sprintf("%d дня", n)
	default:
		return fmt.Sprintf("%d дней", n)
	}
}
