package handlers

import (
	"net/http"
	"strings"

	"version20/internal/achievements"
	"version20/internal/leveling"
	"version20/internal/models"
	"version20/internal/quotes"
	"version20/internal/reports"
	"version20/internal/subscription"
)

type HomeData struct {
	Name              string
	DateLabel         string // "Суббота, 26 сентября"
	DayNumber         int    // день пути: выполнено + 1 (если сегодня уже выполнено — выполнено)
	StreakCurrent     int
	Level             int
	XP                int
	ProgressPct       float64
	Category          string
	TaskText          string
	TaskWhy           string
	Finished          bool
	CanActToday       bool
	DoneToday         bool
	FocusAreas        []string
	Celebrate         *achievements.Meta
	CelebrateLevel100 bool
	Diamond           bool // уровень 100 — алмаз у имени
	TrialDaysLeft     int  // > 0 — идёт пробный период без подписки
	Quote             string
	Week              []WeekDay
	JustDone          bool // ?done=1 — только что закрыл день: конфетти (этап 10)
}

func (a *App) renderHome(w http.ResponseWriter, r *http.Request, user *models.User) {
	now := a.now()
	progressPct, level := leveling.ProgressFromCompleted(user.CompletedCount)
	today := now.In(reports.MoscowLocation()).Format("2006-01-02")
	doneToday := user.LastActionDate.Valid && user.LastActionDate.String == today

	data := HomeData{
		Name:          user.Name,
		DateLabel:     longDate(now.In(reports.MoscowLocation())),
		DayNumber:     user.CompletedCount + 1,
		StreakCurrent: user.StreakCurrent,
		Level:         level,
		XP:            user.XP,
		ProgressPct:   progressPct,
		CanActToday:   !doneToday,
		Diamond:       user.Level >= 100,
		JustDone:      r.URL.Query().Get("done") == "1",
	}
	if doneToday && user.CompletedCount > 0 {
		data.DayNumber = user.CompletedCount
	}
	if data.DayNumber > 365 {
		data.DayNumber = 365
	}
	if subscription.ActiveTier(user.SubscriptionTier, user.SubscriptionExpiresAt) == "" {
		data.TrialDaysLeft = subscription.TrialDaysLeft(user.CreatedAt, now)
	}

	if focus := r.URL.Query().Get("focus"); focus != "" {
		data.FocusAreas = strings.Split(focus, ",")
	}
	if c := r.URL.Query().Get("celebrate"); c != "" {
		if c == "level100" {
			data.CelebrateLevel100 = true
		} else if meta := achievements.MetaByCode(c); meta.Name != "" {
			data.Celebrate = &meta
		}
	}

	actions, err := a.Store.ActionsSince(user.ID, weekStart(now).Format("2006-01-02"))
	if err != nil {
		a.serverError(w, err)
		return
	}
	data.Week = buildWeek(now, actions)
	data.DoneToday = actions[today] == "done"

	if user.DayIndex >= 365 {
		data.Finished = true
		data.Quote = quotes.ForUser(user.TgID, today, "")
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
	data.Quote = quotes.ForUser(user.TgID, today, row.Category)

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
