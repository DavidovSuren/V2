// Package achievements — разблокировка бейджей. Порт backend/lib/achievements.js.
package achievements

import (
	"strconv"
	"time"

	"version20/internal/leveling"
	"version20/internal/models"
)

type Meta struct {
	Code string
	Icon string
	Name string
	Desc string
}

var Metas = []Meta{
	{"7d", "🔥", "7 ДНЕЙ", "7 дней подряд без пропусков"},
	{"30d", "💪", "30 ДНЕЙ", "30 дней подряд без пропусков"},
	{"180d", "🚀", "180 ДНЕЙ", "180 дней подряд без пропусков"},
	{"365d", "👑", "365 ДНЕЙ", "Целый год без пропусков"},
	{"lvl25", "⭐", "УРОВЕНЬ 25", "Четверть пути пройдена"},
	{"lvl50", "🌟", "УРОВЕНЬ 50", "Половина пути пройдена"},
	{"lvl75", "✨", "УРОВЕНЬ 75", "Три четверти пути"},
	{"lvl100", "💎", "УРОВЕНЬ 100", "Version 2.0 полностью собрана"},
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

// CheckAndUnlock проверяет стрик и уровень пользователя и открывает новые
// бейджи. Возвращает список кодов, открытых именно сейчас.
func CheckAndUnlock(c checker, u *models.User) ([]string, error) {
	var unlocked []string
	now := time.Now().UTC().Format(time.RFC3339)

	type milestone struct {
		code    string
		reached bool
	}
	var all []milestone
	for _, d := range leveling.StreakMilestones {
		all = append(all, milestone{code: strconv.Itoa(d) + "d", reached: u.StreakCurrent >= d})
	}
	for _, l := range leveling.LevelMilestones {
		all = append(all, milestone{code: "lvl" + strconv.Itoa(l), reached: u.Level >= l})
	}

	for _, m := range all {
		if !m.reached {
			continue
		}
		has, err := c.HasAchievement(u.ID, m.code)
		if err != nil {
			return nil, err
		}
		if has {
			continue
		}
		if err := c.InsertAchievement(u.ID, m.code, now); err != nil {
			return nil, err
		}
		unlocked = append(unlocked, m.code)
	}
	return unlocked, nil
}
