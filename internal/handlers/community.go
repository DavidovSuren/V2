package handlers

import (
	"net/http"
	"strings"
	"time"

	"version20/internal/achievements"
	"version20/internal/store"
	"version20/internal/subscription"
)

type CommunityData struct {
	Tab          string
	PremiumView  bool
	Upsell       string
	People       []store.PersonRow
	AddFriendErr string
	MeID         int64
	HiddenCount  int // сколько участников скрыто от не-Premium
}

// handleCommunity — раздел "Сообщество". Виден всем, но уровень/бейджи/
// алмаз видны только с активной подпиской 888 ₽. Порт backend/routes/community.js.
func (a *App) handleCommunity(w http.ResponseWriter, r *http.Request) {
	user := userFromCtx(r)
	tab := r.URL.Query().Get("tab")
	premium := subscription.IsPremiumActive(user.SubscriptionTier, user.SubscriptionExpiresAt)

	data := CommunityData{Tab: tab, PremiumView: premium, MeID: user.ID, AddFriendErr: friendAddErrors[r.URL.Query().Get("err")]}

	if tab == "friends" {
		if !premium {
			data.Upsell = "Раздел «Контакты» доступен только с подпиской 888 ₽/мес"
			a.render(w, "community.html", data)
			return
		}
		people, err := a.Store.FriendsRaw(user.ID)
		if err == nil {
			people, err = attachBadgeIcons(a.Store, people)
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
		people, err = attachBadgeIcons(a.Store, people)
		if err != nil {
			a.serverError(w, err)
			return
		}
		data.People = people
	} else {
		// Без Premium видно только себя — остальные участники скрыты.
		for _, p := range people {
			if p.ID == user.ID {
				data.People = append(data.People, p)
			}
		}
		data.HiddenCount = len(people) - len(data.People)
		data.Upsell = "Оформи подписку 888 ₽/мес, чтобы видеть всех участников, их уровень, стрик и достижения 🔒"
	}

	a.render(w, "community.html", data)
}

func attachBadgeIcons(st *store.Store, people []store.PersonRow) ([]store.PersonRow, error) {
	people, err := st.AttachBadges(people)
	if err != nil {
		return nil, err
	}
	for i := range people {
		for _, code := range people[i].Badges {
			if icon := achievements.MetaByCode(code).Icon; icon != "" {
				people[i].BadgeIcons = append(people[i].BadgeIcons, icon)
			}
		}
	}
	return people, nil
}

// Тексты ошибок — те же, что отдавал POST /api/friends/add в Node-версии.
var friendAddErrors = map[string]string{
	"notfound": "Такой пользователь не найден в приложении",
	"self":     "Нельзя добавить самого себя",
}

func (a *App) handleFriendAdd(w http.ResponseWriter, r *http.Request) {
	user := userFromCtx(r)
	if !subscription.IsPremiumActive(user.SubscriptionTier, user.SubscriptionExpiresAt) {
		http.Redirect(w, r, "/community?tab=friends", http.StatusSeeOther)
		return
	}
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
	if friend == nil {
		http.Redirect(w, r, "/community?tab=friends&err=notfound", http.StatusSeeOther)
		return
	}
	if friend.ID == user.ID {
		http.Redirect(w, r, "/community?tab=friends&err=self", http.StatusSeeOther)
		return
	}

	now := time.Now().UTC().Format(time.RFC3339)
	_ = a.Store.AddFriendship(user.ID, friend.ID, now)
	_ = a.Store.AddFriendship(friend.ID, user.ID, now)

	http.Redirect(w, r, "/community?tab=friends", http.StatusSeeOther)
}
