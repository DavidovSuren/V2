package handlers

import (
	"math"
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

type AchievementItem struct {
	achievements.Meta
	Unlocked bool
}

// ProgressData — экран «Путь» (макет 04): уровень, направления и награды.
type ProgressData struct {
	Level          int
	XP             int
	CompletedCount int
	TotalTasks     int
	StreakCurrent  int
	StreakBest     int
	Categories     []CategoryProgress
	Awards         []AchievementItem
	AwardsUnlocked int
	RingDash       float64 // длина окружности кольца уровня
	RingOffset     float64
}

const ringRadius = 36

func (a *App) handleProgress(w http.ResponseWriter, r *http.Request) {
	user := userFromCtx(r)
	_, level := leveling.ProgressFromCompleted(user.CompletedCount)

	doneRows, err := a.Store.DoneCountsByCategory(user.ID)
	if err != nil {
		a.serverError(w, err)
		return
	}
	doneByCategory := map[string]int{}
	for _, d := range doneRows {
		doneByCategory[d.Category] = d.Done
	}

	var categories []CategoryProgress
	for _, ct := range a.CategoryTotals(user.Gender.String) {
		done := doneByCategory[ct.Category]
		categories = append(categories, CategoryProgress{
			Name: ct.Category, Total: ct.Total, Done: done,
			Pct: leveling.CategoryProgressPct(done, ct.Total),
		})
	}

	rows, err := a.Store.UserAchievements(user.ID)
	if err != nil {
		a.serverError(w, err)
		return
	}
	unlocked := map[string]bool{}
	for _, row := range rows {
		unlocked[row.Code] = true
	}

	dash := 2 * math.Pi * ringRadius
	data := ProgressData{
		Level: level, XP: user.XP, CompletedCount: user.CompletedCount,
		TotalTasks: leveling.TotalTasks, StreakCurrent: user.StreakCurrent,
		StreakBest: user.StreakBest, Categories: categories,
		RingDash: math.Round(dash*10) / 10, RingOffset: math.Round(dash*(1-float64(level)/100)*10) / 10,
	}
	for _, m := range achievements.Metas {
		data.Awards = append(data.Awards, AchievementItem{Meta: m, Unlocked: unlocked[m.Code]})
		if unlocked[m.Code] {
			data.AwardsUnlocked++
		}
	}

	a.render(w, "progress.html", data)
}

// handleAchievements — награды теперь на экране «Путь».
func (a *App) handleAchievements(w http.ResponseWriter, r *http.Request) {
	http.Redirect(w, r, "/progress#awards", http.StatusMovedPermanently)
}
