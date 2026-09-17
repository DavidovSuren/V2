// Package cron — напоминание 15:15 МСК + еженедельный/ежемесячный отчёт.
// Порт backend/cron/*.js.
package cron

import (
	"fmt"
	"log"
	"strconv"
	"time"

	"github.com/robfig/cron/v3"

	"version20/internal/leveling"
	"version20/internal/reports"
	"version20/internal/store"
	"version20/internal/telegram"
)

func mskLocation() *time.Location {
	loc, err := time.LoadLocation("Europe/Moscow")
	if err != nil {
		return time.FixedZone("MSK", 3*60*60)
	}
	return loc
}

func Start(s *store.Store, botToken string) {
	c := cron.New(cron.WithLocation(mskLocation()))

	c.AddFunc("15 15 * * *", func() { dailyReminder(s, botToken) })
	c.AddFunc("0 10 * * 1", func() { weeklyReport(s, botToken) })
	c.AddFunc("0 11 1 * *", func() { monthlyReport(s, botToken) })

	c.Start()
	log.Println("[cron] dailyReminder запланирован на 15:15 Europe/Moscow")
	log.Println("[cron] weeklyReport запланирован на понедельник 10:00 Europe/Moscow")
	log.Println("[cron] monthlyReport запланирован на 1-е число 11:00 Europe/Moscow")
}

func dailyReminder(s *store.Store, botToken string) {
	today := reports.TodayMoscow()
	users, err := s.UsersPendingToday(today)
	if err != nil {
		log.Println("[cron] dailyReminder error:", err)
		return
	}
	for _, u := range users {
		row, err := s.GetScheduleRow(u.ID, u.DayIndex)
		if err != nil || row == nil {
			continue
		}
		telegram.SendMessage(botToken, u.TgID, "⏰ Напоминание Version 2.0\n\nСегодняшнее действие:\n"+row.Text+
			"\n\nОткрой приложение и отметь, выполнил(а) ли ты его.")
	}
}

func weeklyReport(s *store.Store, botToken string) {
	from := reports.FromNDaysAgo(6)
	users, err := s.AllQuizzedUsers()
	if err != nil {
		log.Println("[cron] weeklyReport error:", err)
		return
	}
	for _, u := range users {
		done, skipped, err := s.ActionCounts(u.ID, from)
		if err != nil {
			continue
		}
		topCategory, _ := s.TopDoneCategory(u.ID, from)
		focusNext, _ := s.TopWeightedRemainingCategory(u.ID)
		_ = s.AddXP(u.ID, leveling.WeeklyReportXP)

		text := fmt.Sprintf("📊 Недельный отчёт\n\nВыполнено заданий: %d / 7\nПропущено: %d\n", done, skipped)
		if topCategory != "" {
			text += fmt.Sprintf("Главный результат недели: «%s»\n", topCategory)
		}
		if focusNext != "" {
			text += fmt.Sprintf("Фокус следующей недели: «%s»\n", focusNext)
		}
		text += fmt.Sprintf("\n+%d XP за то, что ты не бросил(а). Ты молодец — продолжай в том же духе! 💪", leveling.WeeklyReportXP)

		telegram.SendMessage(botToken, u.TgID, text)
	}
}

func monthlyReport(s *store.Store, botToken string) {
	from := reports.FromNDaysAgo(29)
	users, err := s.AllQuizzedUsers()
	if err != nil {
		log.Println("[cron] monthlyReport error:", err)
		return
	}
	for _, u := range users {
		done, skipped, err := s.ActionCounts(u.ID, from)
		if err != nil {
			continue
		}
		full, err := s.GetUserByID(u.ID)
		if err != nil || full == nil {
			continue
		}
		moodRows, _ := s.MoodRows(u.ID, from)
		entries := make([]reports.MoodEntry, 0, len(moodRows))
		for _, m := range moodRows {
			emoji, _ := strconv.Atoi(m.Emoji)
			entries = append(entries, reports.MoodEntry{EntryDate: m.EntryDate, Emoji: emoji})
		}
		trend := reports.BuildMoodTrend(from, entries)
		_ = s.AddXP(u.ID, leveling.MonthlyReportXP)

		moodLine := "пока недостаточно записей в дневнике"
		if len(trend) > 0 {
			parts := ""
			for i, m := range trend {
				if i > 0 {
					parts += "  "
				}
				parts += fmt.Sprintf("нед.%d: %.1f/5", m.Week, m.AvgMood)
			}
			moodLine = parts
		}

		text := fmt.Sprintf(
			"🗓 Отчёт за месяц\n\nВыполнено заданий: %d\nПропущено: %d\nЛучший стрик: %d дней\nДинамика настроения: %s\n\n"+
				"Где-то не дожал(а) — и это нормально. Ты всё равно на 30 дней ближе к своей Version 2.0. +%d XP за этот месяц! 🚀",
			done, skipped, full.StreakBest, moodLine, leveling.MonthlyReportXP,
		)

		telegram.SendMessage(botToken, u.TgID, text)
	}
}
