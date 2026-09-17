package handlers

import (
	"database/sql"
	"net/http"
	"time"

	"version20/internal/achievements"
	"version20/internal/leveling"
	"version20/internal/reports"
)

type DiaryPageData struct {
	MoodTrend []reports.MoodPoint
}

func (a *App) handleDiaryShow(w http.ResponseWriter, r *http.Request) {
	user := userFromCtx(r)
	from := reports.FromNDaysAgo(29)
	moodRows, _ := a.Store.MoodRows(user.ID, from)

	entries := make([]reports.MoodEntry, 0, len(moodRows))
	for _, m := range moodRows {
		entries = append(entries, reports.MoodEntry{EntryDate: m.EntryDate, Emoji: atoiSafe(m.Emoji)})
	}

	a.render(w, "diary.html", DiaryPageData{MoodTrend: reports.BuildMoodTrend(from, entries)})
}

func atoiSafe(s string) int {
	n := 0
	for _, c := range s {
		if c < '0' || c > '9' {
			return n
		}
		n = n*10 + int(c-'0')
	}
	return n
}

var validEmoji = map[string]bool{"1": true, "2": true, "3": true, "4": true, "5": true}

// handleDiarySubmit — сохранение дневника ЭТО ЖЕ И ЕСТЬ "выполнил(а)
// задание": кнопка "Выполнил(а)" на главном экране просто ведёт на /diary,
// а сам прогресс засчитывается именно тут. Порт backend/routes/diary.js (POST /diary).
func (a *App) handleDiarySubmit(w http.ResponseWriter, r *http.Request) {
	user := userFromCtx(r)
	r.ParseForm()
	emoji := r.FormValue("emoji")
	note := r.FormValue("note")

	if !validEmoji[emoji] {
		http.Redirect(w, r, "/diary", http.StatusSeeOther)
		return
	}

	today := reports.TodayMoscow()
	if user.DayIndex >= 365 || (user.LastActionDate.Valid && user.LastActionDate.String == today) {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}

	row, err := a.Store.GetScheduleRow(user.ID, user.DayIndex)
	if err != nil || row == nil {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}

	newStreak := 1
	if isYesterday(user.LastActionDate, today) {
		newStreak = user.StreakCurrent + 1
	}
	streakBest := user.StreakBest
	if newStreak > streakBest {
		streakBest = newStreak
	}
	completedCount := user.CompletedCount + 1
	xpGain := leveling.XPForTaskCompletion(newStreak)
	_, level := leveling.ProgressFromCompleted(completedCount)
	wasBelow100 := user.Level < 100

	tx, err := a.Store.DB.Begin()
	if err != nil {
		a.serverError(w, err)
		return
	}
	defer tx.Rollback()

	if err := a.Store.UpdateScheduleStatus(tx, user.ID, user.DayIndex, "done"); err != nil {
		a.serverError(w, err)
		return
	}
	noteVal := sql.NullString{String: note, Valid: note != ""}
	if err := a.Store.InsertDiaryEntry(tx, user.ID, today, emoji, noteVal.String, xpGain); err != nil {
		a.serverError(w, err)
		return
	}
	if err := a.Store.InsertActionLog(tx, user.ID, today, "done", row.Category); err != nil {
		a.serverError(w, err)
		return
	}
	if err := a.Store.ApplyCompletion(tx, user.ID, completedCount, xpGain, level, newStreak, streakBest, today); err != nil {
		a.serverError(w, err)
		return
	}
	reachedLevel100 := level >= 100 && wasBelow100
	if reachedLevel100 {
		expires := time.Now().UTC().AddDate(0, 6, 0).Format(time.RFC3339)
		if err := a.Store.GrantLevel100Premium(tx, user.ID, expires); err != nil {
			a.serverError(w, err)
			return
		}
	}
	if err := tx.Commit(); err != nil {
		a.serverError(w, err)
		return
	}

	updatedUser, err := a.Store.GetUserByID(user.ID)
	if err == nil && updatedUser != nil {
		unlocked, _ := achievements.CheckAndUnlock(a.Store, updatedUser)
		redirectURL := "/"
		if reachedLevel100 {
			redirectURL = "/?celebrate=level100"
		} else if len(unlocked) > 0 {
			redirectURL = "/?celebrate=" + unlocked[0]
		}
		http.Redirect(w, r, redirectURL, http.StatusSeeOther)
		return
	}

	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func isYesterday(lastActionDate sql.NullString, today string) bool {
	if !lastActionDate.Valid {
		return false
	}
	d, err1 := time.Parse("2006-01-02", lastActionDate.String)
	t, err2 := time.Parse("2006-01-02", today)
	if err1 != nil || err2 != nil {
		return false
	}
	return t.Sub(d).Hours() == 24
}
