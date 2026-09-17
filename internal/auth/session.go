// Package auth — сессии поверх Telegram initData. Первый заход отдаёт
// маленький бутстрап-скрипт (web/static/bootstrap.js), который читает
// Telegram.WebApp.initData и шлёт его на /auth/bootstrap; сервер проверяет
// подпись (telegram.VerifyInitData) и заводит подписанную cookie — дальше
// вся навигация идёт обычными ссылками/формами без единого fetch.
package auth

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"net/http"
	"strings"
	"time"
)

const CookieName = "v2_session"

type Sessions struct {
	secret []byte
}

func NewSessions(secret string) *Sessions {
	return &Sessions{secret: []byte(secret)}
}

func (s *Sessions) sign(tgID string) string {
	mac := hmac.New(sha256.New, s.secret)
	mac.Write([]byte(tgID))
	sig := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	return tgID + "." + sig
}

// Verify возвращает tg_id из подписанного токена, если подпись верна.
func (s *Sessions) Verify(token string) (string, bool) {
	parts := strings.SplitN(token, ".", 2)
	if len(parts) != 2 {
		return "", false
	}
	tgID := parts[0]
	expected := s.sign(tgID)
	if hmac.Equal([]byte(token), []byte(expected)) {
		return tgID, true
	}
	return "", false
}

func (s *Sessions) SetCookie(w http.ResponseWriter, tgID string) {
	http.SetCookie(w, &http.Cookie{
		Name:     CookieName,
		Value:    s.sign(tgID),
		Path:     "/",
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteLaxMode,
		Expires:  time.Now().Add(180 * 24 * time.Hour),
	})
}

func (s *Sessions) ClearCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     CookieName,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		MaxAge:   -1,
	})
}

// TgIDFromRequest достаёт и проверяет cookie сессии текущего запроса.
func (s *Sessions) TgIDFromRequest(r *http.Request) (string, bool) {
	c, err := r.Cookie(CookieName)
	if err != nil {
		return "", false
	}
	return s.Verify(c.Value)
}
