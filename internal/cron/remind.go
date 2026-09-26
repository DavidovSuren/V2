package cron

import (
	"log"
	"strings"
	"time"

	"version20/internal/quotes"
	"version20/internal/reminders"
	"version20/internal/store"
	"version20/internal/telegram"
)

// send — все сообщения бота из фоновых задач: с кнопкой, открывающей Mini
// App на нужной странице (если задан PUBLIC_URL), иначе обычным текстом.
func send(cfg Config, tgID, text, button, path string) {
	if cfg.BotToken == "" {
		return
	}
	if cfg.PublicURL == "" {
		telegram.SendMessage(cfg.BotToken, tgID, text)
		return
	}
	if err := telegram.NewBot(cfg.BotToken).SendMessageWithWebAppButton(tgID, text, button,
		strings.TrimRight(cfg.PublicURL, "/")+path); err != nil {
		log.Println("[cron] send:", err)
	}
}

func dailyText(task, quote string) string {
	return "Сегодняшнее действие:\n" + task + "\n\nЦитата дня: «" + quote + "»"
}

func eveningText(streak int, task string) string {
	return "Стрик " + reminders.DaysWord(streak) + " сгорит в полночь. Одно действие — и он продолжится.\n\n" + task
}

// remindTick — раз в минуту: ежедневное напоминание тем, чьё время
// наступило, и в 21:00 — тем, у кого стрик от 3 дней, а сегодня ещё ничего.
func remindTick(s *store.Store, cfg Config, now time.Time) {
	if cfg.BotToken == "" {
		return
	}
	msk := mskLocation()
	clock := reminders.Clock(now, msk)
	today := now.In(msk).Format("2006-01-02")
	users, err := s.UsersPendingToday(today)
	if err != nil {
		log.Println("[cron] reminders error:", err)
		return
	}
	for _, u := range users {
		daily := reminders.Due(u.RemindAt, clock)
		evening := reminders.EveningDue(u.Streak, clock)
		if !daily && !evening {
			continue
		}
		row, err := s.GetScheduleRow(u.ID, u.DayIndex)
		if err != nil || row == nil {
			continue
		}
		if evening {
			send(cfg, u.TgID, eveningText(u.Streak, row.Text), "Открыть", "/")
			continue
		}
		send(cfg, u.TgID, dailyText(row.Text, quotes.ForUser(u.TgID, today, row.Category)), "Открыть", "/")
	}
}
