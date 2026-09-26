// Package achievements — разблокировка наград. Порт backend/lib/achievements.js,
// плюс «Первый шаг» и «Уровень 10» (этап 7): всего 10 наград.
package achievements

import (
	"time"

	"version20/internal/models"
)

type Meta struct {
	Code string
	Icon string // id иконки в SVG-спрайте (partials/icons.html): flame, star, check, diamond
	Tone string // цвет кружка: flame, green, violet, gold
	Name string
	Desc string
}

// Metas — в порядке показа на экране «Путь».
var Metas = []Meta{
	{"first", "check", "violet", "Первый шаг", "Первое выполненное задание"},
	{"7d", "flame", "flame", "7 дней", "7 дней подряд без пропусков"},
	{"lvl10", "star", "green", "Уровень 10", "Первые 10 уровней позади"},
	{"30d", "flame", "flame", "30 дней", "30 дней подряд без пропусков"},
	{"lvl25", "star", "green", "Уровень 25", "Четверть пути пройдена"},
	{"lvl50", "star", "green", "Уровень 50", "Половина пути пройдена"},
	{"180d", "flame", "flame", "180 дней", "180 дней подряд без пропусков"},
	{"lvl75", "star", "green", "Уровень 75", "Три четверти пути"},
	{"365d", "flame", "flame", "365 дней", "Целый год без пропусков"},
	{"lvl100", "diamond", "gold", "Алмаз", "Уровень 100 — Version 2.0 полностью собрана"},
}

// reached — условие каждой награды.
func reached(code string, u *models.User) bool {
	switch code {
	case "first":
		return u.CompletedCount >= 1
	case "7d":
		return u.StreakCurrent >= 7
	case "30d":
		return u.StreakCurrent >= 30
	case "180d":
		return u.StreakCurrent >= 180
	case "365d":
		return u.StreakCurrent >= 365
	case "lvl10":
		return u.Level >= 10
	case "lvl25":
		return u.Level >= 25
	case "lvl50":
		return u.Level >= 50
	case "lvl75":
		return u.Level >= 75
	case "lvl100":
		return u.Level >= 100
	}
	return false
}

func MetaByCode(code string) Meta {
	for _, m := range Metas {
		if m.Code == code {
			return m
		}
	}
	return Meta{Code: code}
}

type checker interface {
	HasAchievement(userID int64, code string) (bool, error)
	InsertAchievement(userID int64, code, unlockedAt string) error
}

// CheckAndUnlock открывает все заслуженные, но ещё не открытые награды.
// Возвращает коды, открытые именно сейчас, в порядке Metas.
func CheckAndUnlock(c checker, u *models.User) ([]string, error) {
	var unlocked []string
	now := time.Now().UTC().Format(time.RFC3339)
	for _, m := range Metas {
		if !reached(m.Code, u) {
			continue
		}
		has, err := c.HasAchievement(u.ID, m.Code)
		if err != nil {
			return nil, err
		}
		if has {
			continue
		}
		if err := c.InsertAchievement(u.ID, m.Code, now); err != nil {
			return nil, err
		}
		unlocked = append(unlocked, m.Code)
	}
	return unlocked, nil
}
