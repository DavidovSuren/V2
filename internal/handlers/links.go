package handlers

import (
	"net/url"
	"strings"
)

// shareText — текст, с которым друг получает приглашение.
const shareText = "Я прокачиваю себя по одному действию в день в Version 2.0. Присоединяйся — 3 дня бесплатно"

// referralLink — ссылка-приглашение: Mini App напрямую, если задан
// MINIAPP_SHORT_NAME (t.me/<бот>/<имя>?startapp=КОД), иначе через /start
// бота (t.me/<бот>?start=КОД). Без @username бота — адрес приложения с ?ref=.
func (a *App) referralLink(code string) string {
	if code == "" {
		return ""
	}
	if a.BotUsername != "" {
		if a.MiniAppShortName != "" {
			return "https://t.me/" + a.BotUsername + "/" + a.MiniAppShortName + "?startapp=" + url.QueryEscape(code)
		}
		return "https://t.me/" + a.BotUsername + "?start=" + url.QueryEscape(code)
	}
	if a.PublicURL != "" {
		return strings.TrimRight(a.PublicURL, "/") + "/?ref=" + url.QueryEscape(code)
	}
	return code
}

// shareURL — t.me/share/url, открывается через Telegram.WebApp.openTelegramLink.
func shareURL(link string) string {
	if link == "" {
		return ""
	}
	return "https://t.me/share/url?" + url.Values{"url": {link}, "text": {shareText}}.Encode()
}
