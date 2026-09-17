package handlers

import (
	"net/http"
	"strings"

	"version20/internal/achievements"
	"version20/internal/leveling"
	"version20/internal/models"
	"version20/internal/reports"
)

type HomeData struct {
	ProgressPct       float64
	CircleOffset      float64
	DayLabel          string
	Category          string
	TaskText          string
	TaskWhy           string
	Finished          bool
	CanActToday       bool
	FocusAreas        []string
	Celebrate         *achievements.Meta
	CelebrateLevel100 bool
}

func (a *App) renderHome(w http.ResponseWriter, r *http.Request, user *models.User) {
	progressPct, _ := leveling.ProgressFromCompleted(user.CompletedCount)
	circumference := 326.7 // 2*pi*52, совпадает с исходной разметкой
	offset := circumference - (progressPct/100)*circumference

	data := HomeData{
		ProgressPct:  progressPct,
		CircleOffset: offset,
		CanActToday:  !user.LastActionDate.Valid || user.LastActionDate.String != reports.TodayMoscow(),
	}

	if focus := r.URL.Query().Get("focus"); focus != "" {
		data.FocusAreas = strings.Split(focus, ",")
	}
	if c := r.URL.Query().Get("celebrate"); c != "" {
		if c == "level100" {
			data.CelebrateLevel100 = true
		} else {
			meta := achievements.MetaByCode(c)
			data.Celebrate = &meta
		}
	}

	if user.DayIndex >= 365 {
		data.Finished = true
		data.DayLabel = "Путь пройден полностью"
		a.render(w, "home.html", data)
		return
	}

	row, err := a.Store.GetScheduleRow(user.ID, user.DayIndex)
	if err != nil || row == nil {
		a.serverError(w, err)
		return
	}
	data.Category = row.Category
	data.TaskText = row.Text
	data.TaskWhy = row.Why
	data.DayLabel = dayLabel(user.CompletedCount)

	a.render(w, "home.html", data)
}

func dayLabel(completed int) string {
	return "День " + itoa(completed+1) + " из 365"
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b []byte
	for n > 0 {
		b = append([]byte{byte('0' + n%10)}, b...)
		n /= 10
	}
	return string(b)
}

func (a *App) handleSkipConfirm(w http.ResponseWriter, r *http.Request) {
	user := userFromCtx(r)
	if user.DayIndex >= 365 || (user.LastActionDate.Valid && user.LastActionDate.String == reports.TodayMoscow()) {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}
	a.render(w, "skip_confirm.html", nil)
}

func (a *App) handleSkipSubmit(w http.ResponseWriter, r *http.Request) {
	user := userFromCtx(r)
	today := reports.TodayMoscow()

	if user.DayIndex >= 365 || (user.LastActionDate.Valid && user.LastActionDate.String == today) {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}

	row, _ := a.Store.GetScheduleRow(user.ID, user.DayIndex)
	category := ""
	if row != nil {
		category = row.Category
	}

	tx, err := a.Store.DB.Begin()
	if err != nil {
		a.serverError(w, err)
		return
	}
	defer tx.Rollback()

	if err := a.Store.InsertActionLog(tx, user.ID, today, "skip", category); err != nil {
		a.serverError(w, err)
		return
	}
	if _, err := tx.Exec("UPDATE users SET last_action_date = $1, streak_current = 0 WHERE id = $2", today, user.ID); err != nil {
		a.serverError(w, err)
		return
	}
	if err := tx.Commit(); err != nil {
		a.serverError(w, err)
		return
	}

	http.Redirect(w, r, "/", http.StatusSeeOther)
}
