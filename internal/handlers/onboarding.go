package handlers

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"version20/internal/codes"
	"version20/internal/store"
)

// handleOnboardingSubmit — экран "Приветствие". Порт backend/routes/onboarding.js (POST /onboarding).
func (a *App) handleOnboardingSubmit(w http.ResponseWriter, r *http.Request) {
	tgID, ok := a.Sessions.TgIDFromRequest(r)
	if !ok {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}

	_ = r.ParseMultipartForm(2 << 20) // фото — тот же стаб, что в исходном MVP: не сохраняются, важно лишь количество
	name := strings.TrimSpace(r.FormValue("name"))
	ageGroup := r.FormValue("ageGroup")
	refCode := strings.TrimSpace(r.FormValue("refCode"))
	photoCount := 0
	if r.MultipartForm != nil {
		photoCount = len(r.MultipartForm.File["photos"])
	}

	if name == "" || ageGroup == "" || photoCount == 0 {
		a.render(w, "welcome.html", WelcomeData{RefCode: refCode, Error: "Заполни имя, возрастную группу и загрузи хотя бы одно фото"})
		return
	}

	existing, err := a.Store.GetUserByTgID(tgID)
	if err != nil {
		a.serverError(w, err)
		return
	}

	photosJSON, _ := json.Marshal(placeholderPhotos(photoCount))

	if existing != nil {
		if err := a.Store.UpdateProfile(existing.ID, name, ageGroup, string(photosJSON)); err != nil {
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

	newUser := store.NewUser{
		TgID:               tgID,
		Name:               name,
		AgeGroup:           ageGroup,
		PhotosJSON:         string(photosJSON),
		CreatedAt:          time.Now().UTC().Format(time.RFC3339),
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

// placeholderPhotos — реальная загрузка/хранение фото не реализована (как
// и в исходном MVP), но UX ждёт "минимум 1 фото" — фиксируем только количество.
func placeholderPhotos(n int) []string {
	out := make([]string, n)
	for i := range out {
		out[i] = "local-photo"
	}
	return out
}
