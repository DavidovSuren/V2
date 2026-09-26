package cron

import (
	"database/sql"
	"log"
	"time"

	"version20/internal/store"
	"version20/internal/subscription"
)

const trialEndingText = "Завтра заканчивается пробный период Version 2.0. " +
	"Чтобы не потерять стрик и продолжить путь, выбери тариф — Plus или Premium."

// trialEndsTomorrow — пробный период закончится в ближайшие сутки, а
// подписки нет. Задача запускается раз в день, так что каждый попадает в
// это окно ровно один раз.
func trialEndsTomorrow(createdAt, tier string, expiresAt sql.NullString, now time.Time) bool {
	if subscription.IsAnySubscriptionActive(tier, expiresAt) {
		return false
	}
	ends := subscription.TrialEnds(createdAt)
	return ends.After(now) && !ends.After(now.Add(24*time.Hour))
}

func trialEnding(s *store.Store, cfg Config, now time.Time) {
	if cfg.BotToken == "" {
		return
	}
	since := now.Add(-time.Duration(subscription.TrialDays+1) * 24 * time.Hour).UTC().Format(time.RFC3339)
	users, err := s.UsersRegisteredSince(since)
	if err != nil {
		log.Println("[cron] trialEnding error:", err)
		return
	}
	for _, u := range users {
		if trialEndsTomorrow(u.CreatedAt, u.Tier, u.ExpiresAt, now) {
			send(cfg, u.TgID, trialEndingText, "Выбрать тариф", "/plans")
		}
	}
}
