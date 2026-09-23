package handlers

import (
	"io"
	"net/http"

	"version20/internal/telegram"
)

const (
	refCookieName      = "v2_ref"
	usernameCookieName = "v2_username"
)

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
		if err := a.syncUsername(w, identity); err != nil {
			a.serverError(w, err)
			return
		}
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

// syncUsername — как requireUser в Node-версии: @username нужен для
// добавления друзей и выдачи премиум-агентского кода по @username.
// Зарегистрированному пользователю обновляем его сразу; ещё не
// зарегистрированному — кладём в короткую cookie до POST /onboarding.
func (a *App) syncUsername(w http.ResponseWriter, identity *telegram.Identity) error {
	if identity.Username == "" {
		return nil
	}
	user, err := a.Store.GetUserByTgID(identity.ID)
	if err != nil {
		return err
	}
	if user == nil {
		http.SetCookie(w, &http.Cookie{
			Name: usernameCookieName, Value: identity.Username, Path: "/",
			MaxAge: 3600, HttpOnly: true, SameSite: http.SameSiteLaxMode,
		})
		return nil
	}
	if user.Username.String != identity.Username {
		return a.Store.UpdateUsername(user.ID, identity.Username)
	}
	return nil
}
