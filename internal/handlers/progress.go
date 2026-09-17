package handlers

import (
	"net/http"

	"version20/internal/achievements"
	"version20/internal/leveling"
)

type CategoryProgress struct {
	Name  string
	Pct   int
	Done  int
	Total int
}

type ProgressData struct {
	Level          int
	XP             int
	CompletedCount int
	TotalTasks     int
	StreakCurrent  int
	StreakBest     int
	Categories     []CategoryProgress
}

func (a *App) handleProgress(w http.ResponseWriter, r *http.Request) {
	user := userFromCtx(r)
	progressPct, level := leveling.ProgressFromCompleted(user.CompletedCount)
	_ = progressPct

	doneRows, err := a.Store.DoneCountsByCategory(user.ID)
	if err != nil {
		a.serverError(w, err)
		return
	}
	doneByCategory := map[string]int{}
	for _, d := range doneRows {
		doneByCategory[d.Category] = d.Done
	}

	totals := a.CategoryTotals(user.Gender.String)
	var categories []CategoryProgress
	for cat, total := range totals {
		categories = append(categories, CategoryProgress{
			Name: cat, Total: total, Done: doneByCategory[cat],
			Pct: leveling.CategoryProgressPct(doneByCategory[cat], total),
		})
	}

	a.render(w, "progress.html", ProgressData{
		Level: level, XP: user.XP, CompletedCount: user.CompletedCount,
		TotalTasks: leveling.TotalTasks, StreakCurrent: user.StreakCurrent,
		StreakBest: user.StreakBest, Categories: categories,
	})
}

type AchievementItem struct {
	achievements.Meta
	Unlocked bool
}

func (a *App) handleAchievements(w http.ResponseWriter, r *http.Request) {
	user := userFromCtx(r)
	rows, err := a.Store.UserAchievements(user.ID)
	if err != nil {
		a.serverError(w, err)
		return
	}
	unlockedSet := map[string]bool{}
	for _, row := range rows {
		unlockedSet[row.Code] = true
	}

	items := make([]AchievementItem, 0, len(achievements.Metas))
	for _, m := range achievements.Metas {
		items = append(items, AchievementItem{Meta: m, Unlocked: unlockedSet[m.Code]})
	}

	a.render(w, "achievements.html", items)
}
