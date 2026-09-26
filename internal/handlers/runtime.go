package handlers

import (
	"time"

	"version20/internal/telegram"
)

// now — текущее время; в тестах подменяется через App.Now (например, 15-е число).
func (a *App) now() time.Time {
	if a.Now != nil {
		return a.Now()
	}
	return time.Now()
}

// notify — сообщение пользователю от бота. В тестах App.Notify перехватывает
// сообщения; без BOT_TOKEN ничего не отправляется.
func (a *App) notify(tgID, text string) {
	if tgID == "" {
		return
	}
	if a.Notify != nil {
		a.Notify(tgID, text)
		return
	}
	if a.BotToken == "" {
		return
	}
	go telegram.SendMessage(a.BotToken, tgID, text)
}
