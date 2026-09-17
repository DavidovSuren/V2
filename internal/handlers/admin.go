package handlers

import (
	"net/http"
	"strings"

	"version20/internal/codes"
	"version20/internal/models"
)

func (a *App) lookupTarget(username, tgID string) (*models.User, error) {
	if tgID != "" {
		return a.Store.GetUserByTgID(tgID)
	}
	return a.Store.GetUserByUsername(username)
}

// handleAdminGrantPremiumAgent — только владелец (ADMIN_TG_ID) может выдать
// премиум-агентский код: держатель получает 50% с оплат приглашённых, те
// платят полную цену. Порт backend/routes/admin.js.
func (a *App) handleAdminGrantPremiumAgent(w http.ResponseWriter, r *http.Request) {
	tgID, ok := a.Sessions.TgIDFromRequest(r)
	if !ok || a.AdminTgID == "" || tgID != a.AdminTgID {
		http.Error(w, "Доступно только владельцу приложения", http.StatusForbidden)
		return
	}

	r.ParseForm()
	username := strings.TrimPrefix(strings.TrimSpace(r.FormValue("username")), "@")
	targetTgID := strings.TrimSpace(r.FormValue("tgId"))
	if username == "" && targetTgID == "" {
		http.Error(w, "Укажи username или tgId пользователя", http.StatusBadRequest)
		return
	}

	target, err := a.lookupTarget(username, targetTgID)
	if err != nil {
		a.serverError(w, err)
		return
	}
	if target == nil {
		http.Error(w, "Пользователь не найден", http.StatusNotFound)
		return
	}
	if target.PremiumAgentCode.Valid {
		w.Write([]byte("уже выдан: " + target.PremiumAgentCode.String))
		return
	}

	code := codes.Generate(8)
	for {
		exists, err := a.Store.PremiumAgentCodeExists(code)
		if err != nil {
			a.serverError(w, err)
			return
		}
		if !exists {
			break
		}
		code = codes.Generate(8)
	}

	if err := a.Store.SetPremiumAgentCode(target.ID, code); err != nil {
		a.serverError(w, err)
		return
	}
	w.Write([]byte("код выдан: " + code))
}
