package handlers

import (
	"io"
	"net/http"

	"version20/internal/telegram"
)

const refCookieName = "v2_ref"

// handleAuthBootstrap — единственный fetch() во всём приложении: bootstrap.js
// на первом заходе шлёт сюда Telegram.WebApp.initData, мы проверяем подпись
// и заводим сессионную cookie. Дальше вся навигация — обычные ссылки/формы.
func (a *App) handleAuthBootstrap(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(io.LimitReader(r.Body, 8192))
	if err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	initData := string(body)

	if identity, ok := telegram.VerifyInitData(initData, a.BotToken); ok {
		a.Sessions.SetCookie(w, identity.ID)
		if identity.StartParam != "" {
			http.SetCookie(w, &http.Cookie{
				Name: refCookieName, Value: identity.StartParam, Path: "/",
				MaxAge: 3600, SameSite: http.SameSiteLaxMode,
			})
		}
		w.WriteHeader(http.StatusOK)
		return
	}

	// Локальная разработка вне Telegram: DEV_ALLOW_FAKE_AUTH=true и тело
	// вида "debug:<tg_id>" заводит сессию без проверки подписи.
	if a.DevFakeAuth {
		const prefix = "debug:"
		if len(initData) > len(prefix) && initData[:len(prefix)] == prefix {
			a.Sessions.SetCookie(w, initData[len(prefix):])
			w.WriteHeader(http.StatusOK)
			return
		}
	}

	http.Error(w, "Не удалось подтвердить Telegram-пользователя", http.StatusUnauthorized)
}
