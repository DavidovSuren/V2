package handlers

import (
	"net/http"

	"version20/internal/reminders"
)

type ReminderData struct {
	Current string
	Options []string
	Saved   bool
}

// handleReminderShow — выбор времени ежедневного напоминания (по Москве).
func (a *App) handleReminderShow(w http.ResponseWriter, r *http.Request) {
	a.render(w, "reminder.html", ReminderData{
		Current: reminders.Effective(userFromCtx(r).RemindAt.String),
		Options: reminders.Options(),
		Saved:   r.URL.Query().Get("saved") == "1",
	})
}

func (a *App) handleReminderSave(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()
	at := r.FormValue("at")
	if !reminders.Valid(at) {
		http.Redirect(w, r, "/reminder", http.StatusSeeOther)
		return
	}
	if err := a.Store.SetRemindAt(userFromCtx(r).ID, at); err != nil {
		a.serverError(w, err)
		return
	}
	http.Redirect(w, r, "/reminder?saved=1", http.StatusSeeOther)
}
