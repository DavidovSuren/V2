package handlers

import (
	"time"

	"version20/internal/reports"
)

var weekdaysRu = []string{"Воскресенье", "Понедельник", "Вторник", "Среда", "Четверг", "Пятница", "Суббота"}
var weekdaysShort = []string{"Пн", "Вт", "Ср", "Чт", "Пт", "Сб", "Вс"}
var monthsRuGen = []string{"", "января", "февраля", "марта", "апреля", "мая", "июня",
	"июля", "августа", "сентября", "октября", "ноября", "декабря"}

// longDate — "Суббота, 26 сентября".
func longDate(t time.Time) string {
	return weekdaysRu[t.Weekday()] + ", " + itoa(t.Day()) + " " + monthsRuGen[t.Month()]
}

// weekStart — понедельник текущей недели по Москве (00:00).
func weekStart(now time.Time) time.Time {
	msk := now.In(reports.MoscowLocation())
	offset := (int(msk.Weekday()) + 6) % 7 // Пн = 0
	d := msk.AddDate(0, 0, -offset)
	return time.Date(d.Year(), d.Month(), d.Day(), 0, 0, 0, 0, d.Location())
}

// WeekDay — клетка полоски недели на главной.
type WeekDay struct {
	Label string
	State string // done | skip | today | future | missed
	Today bool
}

func buildWeek(now time.Time, actions map[string]string) []WeekDay {
	start := weekStart(now)
	today := now.In(reports.MoscowLocation()).Format("2006-01-02")
	days := make([]WeekDay, 7)
	for i := range days {
		date := start.AddDate(0, 0, i).Format("2006-01-02")
		d := WeekDay{Label: weekdaysShort[i], Today: date == today}
		switch {
		case actions[date] == "done":
			d.State = "done"
		case d.Today:
			d.State = "today"
		case date > today:
			d.State = "future"
		default:
			d.State = "missed" // пропуск или не заходил
		}
		days[i] = d
	}
	return days
}
