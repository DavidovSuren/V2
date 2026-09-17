package handlers

import (
	"net/http"

	"version20/internal/reports"
)

func weeklyFrom() string { return reports.FromNDaysAgo(6) }

type WeeklyReportData struct {
	Completed     int
	Skipped       int
	ActiveDays    int
	StreakCurrent int
	TopCategory   string
	FocusNextWeek string
}

func (a *App) handleWeeklyReport(w http.ResponseWriter, r *http.Request) {
	user := userFromCtx(r)
	from := weeklyFrom()

	done, skipped, err := a.Store.ActionCounts(user.ID, from)
	if err != nil {
		a.serverError(w, err)
		return
	}
	topCategory, _ := a.Store.TopDoneCategory(user.ID, from)
	focusNext, _ := a.Store.TopWeightedRemainingCategory(user.ID)

	a.render(w, "report.html", WeeklyReportData{
		Completed: done, Skipped: skipped, ActiveDays: done + skipped,
		StreakCurrent: user.StreakCurrent, TopCategory: topCategory, FocusNextWeek: focusNext,
	})
}
