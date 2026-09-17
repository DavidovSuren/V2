package handlers

import (
	"net/http"
	"strings"
	"time"

	"version20/internal/store"
	"version20/internal/subscription"
)

type CommunityData struct {
	Tab          string
	PremiumView  bool
	Upsell       string
	People       []store.PersonRow
	AddFriendErr string
}

// handleCommunity — раздел "Сообщество". Виден всем, но уровень/бейджи/
// алмаз видны только с активной подпиской 888 ₽. Порт backend/routes/community.js.
func (a *App) handleCommunity(w http.ResponseWriter, r *http.Request) {
	user := userFromCtx(r)
	tab := r.URL.Query().Get("tab")
	premium := subscription.IsPremiumActive(user.SubscriptionTier, user.SubscriptionExpiresAt)

	data := CommunityData{Tab: tab, PremiumView: premium}

	if tab == "friends" {
		if !premium {
			data.Upsell = "Раздел «Контакты» доступен только с подпиской 888 ₽/мес"
			a.render(w, "community.html", data)
			return
		}
		people, err := a.Store.FriendsRaw(user.ID)
		if err == nil {
			people, err = a.Store.AttachBadges(people)
		}
		if err != nil {
			a.serverError(w, err)
			return
		}
		data.People = people
		a.render(w, "community.html", data)
		return
	}

	people, err := a.Store.AllUsersRanked()
	if err != nil {
		a.serverError(w, err)
		return
	}
	if premium {
		people, err = a.Store.AttachBadges(people)
		if err != nil {
			a.serverError(w, err)
			return
		}
	} else {
		data.Upsell = "Оформи подписку 888 ₽/мес, чтобы видеть уровень и достижения всех участников 🔒"
	}
	data.People = people

	a.render(w, "community.html", data)
}

func (a *App) handleFriendAdd(w http.ResponseWriter, r *http.Request) {
	user := userFromCtx(r)
	r.ParseForm()
	username := strings.TrimPrefix(strings.TrimSpace(r.FormValue("username")), "@")

	if username == "" {
		http.Redirect(w, r, "/community?tab=friends", http.StatusSeeOther)
		return
	}

	friend, err := a.Store.GetUserByUsername(username)
	if err != nil {
		a.serverError(w, err)
		return
	}
	if friend == nil || friend.ID == user.ID {
		http.Redirect(w, r, "/community?tab=friends&err=notfound", http.StatusSeeOther)
		return
	}

	now := time.Now().UTC().Format(time.RFC3339)
	_ = a.Store.AddFriendship(user.ID, friend.ID, now)
	_ = a.Store.AddFriendship(friend.ID, user.ID, now)

	http.Redirect(w, r, "/community?tab=friends", http.StatusSeeOther)
}
