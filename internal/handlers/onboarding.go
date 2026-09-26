package handlers

import (
	"database/sql"
	"net/http"
	"strings"
	"time"

	"version20/internal/codes"
	"version20/internal/store"
)

// handleOnboardingSubmit — экран "Приветствие". Порт backend/routes/onboarding.js (POST /onboarding).
func (a *App) handleOnboardingSubmit(w http.ResponseWriter, r *http.Request) {
	_ = r.ParseMultipartForm(1 << 20)
	name := strings.TrimSpace(r.FormValue("name"))
	ageGroup := r.FormValue("ageGroup")
	refCode := strings.TrimSpace(r.FormValue("refCode"))
	termsOK := r.FormValue("terms") != ""

	// Без сессии (страница открыта не из Telegram) раньше был молчаливый
	// редирект на "/" — форма просто очищалась, и казалось, что ничего не работает.
	tgID, ok := a.Sessions.TgIDFromRequest(r)
	if !ok {
		a.render(w, "welcome.html", WelcomeData{
			RefCode: refCode, Name: name, AgeGroup: ageGroup,
			NeedBootstrap: true, DevAuth: a.DevFakeAuth,
			Error: "Не удалось определить пользователя Telegram. Открой приложение через бота в Telegram.",
		})
		return
	}

	if name == "" || ageGroup == "" {
		a.render(w, "welcome.html", WelcomeData{RefCode: refCode, Name: name, AgeGroup: ageGroup,
			Error: "Напиши, как к тебе обращаться, и выбери возраст"})
		return
	}
	if !termsOK {
		a.render(w, "welcome.html", WelcomeData{RefCode: refCode, Name: name, AgeGroup: ageGroup,
			Error: "Прими соглашение, чтобы начать"})
		return
	}

	existing, err := a.Store.GetUserByTgID(tgID)
	if err != nil {
		a.serverError(w, err)
		return
	}

	// Фото больше не загружаются; колонка photos_json остаётся для старых данных.
	photosJSON := "[]"

	if existing != nil {
		if err := a.Store.UpdateProfile(existing.ID, name, ageGroup, photosJSON); err != nil {
			a.serverError(w, err)
			return
		}
		if err := a.Store.AcceptTerms(existing.ID, TermsVersion, a.now().UTC().Format(time.RFC3339)); err != nil {
			a.serverError(w, err)
			return
		}
		http.Redirect(w, r, "/quiz/0", http.StatusSeeOther)
		return
	}

	var referredBy sql.NullInt64
	var referredByType sql.NullString
	if refCode != "" {
		referrer, err := a.Store.GetUserByReferralOrAgentCode(refCode)
		if err == nil && referrer != nil {
			referredBy = sql.NullInt64{Int64: referrer.ID, Valid: true}
			codeType := "normal"
			if referrer.PremiumAgentCode.Valid && referrer.PremiumAgentCode.String == refCode {
				codeType = "premium_agent"
			}
			referredByType = sql.NullString{String: codeType, Valid: true}
		}
	}

	referralCode := codes.Generate(7)
	for {
		exists, err := a.Store.ReferralCodeExists(referralCode)
		if err != nil {
			a.serverError(w, err)
			return
		}
		if !exists {
			break
		}
		referralCode = codes.Generate(7)
	}

	var username sql.NullString
	if c, err := r.Cookie(usernameCookieName); err == nil && c.Value != "" {
		username = sql.NullString{String: c.Value, Valid: true}
	}

	newUser := store.NewUser{
		TgID:               tgID,
		Username:           username,
		Name:               name,
		AgeGroup:           ageGroup,
		PhotosJSON:         photosJSON,
		TermsVersion:       TermsVersion,
		CreatedAt:          a.now().UTC().Format(time.RFC3339),
		ReferralCode:       referralCode,
		ReferredByUserID:   referredBy,
		ReferredByCodeType: referredByType,
	}
	userID, err := a.Store.CreateUser(newUser)
	if err != nil {
		a.serverError(w, err)
		return
	}

	if referredBy.Valid {
		_ = a.Store.CreateReferral(a.Store.DB, referredBy.Int64, userID, referredByType.String, newUser.CreatedAt)
	}

	http.Redirect(w, r, "/quiz/0", http.StatusSeeOther)
}
