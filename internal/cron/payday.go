package cron

import (
	"fmt"
	"log"
	"strconv"
	"strings"

	"version20/internal/payouts"
	"version20/internal/store"
	"version20/internal/telegram"
)

func paydayText(balance int) string {
	return fmt.Sprintf("Сегодня день вывода: доступно %s ₽. Выведи на карту в кошельке — только сегодня, до полуночи по Москве.",
		groupDigits(balance))
}

func groupDigits(n int) string {
	s := strconv.Itoa(n)
	var b strings.Builder
	for i := range s {
		if i > 0 && (len(s)-i)%3 == 0 {
			b.WriteString(" ")
		}
		b.WriteByte(s[i])
	}
	return b.String()
}

// payday — 15-го в 10:00 МСК всем, у кого на балансе от минимальной суммы вывода.
func payday(s *store.Store, cfg Config) {
	if cfg.BotToken == "" {
		return
	}
	users, err := s.UsersWithBalanceAtLeast(payouts.MinAmount)
	if err != nil {
		log.Println("[cron] payday error:", err)
		return
	}
	bot := telegram.NewBot(cfg.BotToken)
	for _, u := range users {
		if cfg.PublicURL == "" {
			telegram.SendMessage(cfg.BotToken, u.TgID, paydayText(u.Balance))
			continue
		}
		if err := bot.SendMessageWithWebAppButton(u.TgID, paydayText(u.Balance), "Открыть кошелёк",
			strings.TrimRight(cfg.PublicURL, "/")+"/wallet"); err != nil {
			log.Println("[cron] payday:", err)
		}
	}
}
