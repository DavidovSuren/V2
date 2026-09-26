package handlers

import (
	"net/http"
	"sort"
	"strings"
	"time"

	"version20/internal/store"
	"version20/internal/subscription"
)

type CommunityData struct {
	Tab          string // "" — рейтинг недели, "friends" — мои контакты
	PremiumView  bool
	Upsell       string
	People       []FriendRow
	AddFriendErr string
	ShareURL     string
}

// FriendRow — строка «Друзей» (макет 05).
type FriendRow struct {
	Rank     int
	ID       int64
	Name     string
	Initial  string
	Username string
	Level    int
	WeekDone int
	Diamond  bool
	Me       bool
	CanAdd   bool // Premium: добавить в контакты по @username
}

// ratingLimit — сколько участников показывать в рейтинге недели (плюс ты,
// если не попал в верх).
const ratingLimit = 50

// handleCommunity — «Друзья»: рейтинг недели и мои контакты. Имена и
// «N из 7 на этой неделе» видны всем, уровни, алмазы и контакты — только
// с активным Premium.
func (a *App) handleCommunity(w http.ResponseWriter, r *http.Request) {
	user := userFromCtx(r)
	tab := r.URL.Query().Get("tab")
	if tab != "friends" {
		tab = ""
	}
	premium := subscription.IsPremiumActive(user.SubscriptionTier, user.SubscriptionExpiresAt)
	data := CommunityData{Tab: tab, PremiumView: premium, AddFriendErr: friendAddErrors[r.URL.Query().Get("err")],
		ShareURL: shareURL(a.referralLink(user.ReferralCode.String))}
	if !premium {
		data.Upsell = "Premium открывает уровни всех участников и твоих контактов"
	}

	week, err := a.Store.WeekDoneCounts(weekStart(a.now()).Format("2006-01-02"))
	if err != nil {
		a.serverError(w, err)
		return
	}

	var people []store.PersonRow
	if tab == "friends" {
		if !premium {
			a.render(w, "community.html", data)
			return
		}
		people, err = a.Store.FriendsRaw(user.ID)
	} else {
		people, err = a.Store.AllUsersRanked()
	}
	if err != nil {
		a.serverError(w, err)
		return
	}

	// Рейтинг недели: больше выполнено за неделю — выше; при равенстве — уровень.
	sort.SliceStable(people, func(i, j int) bool {
		if week[people[i].ID] != week[people[j].ID] {
			return week[people[i].ID] > week[people[j].ID]
		}
		return people[i].Level > people[j].Level
	})
	for i, p := range people {
		me := p.ID == user.ID
		if tab == "" && i >= ratingLimit && !me {
			continue
		}
		row := FriendRow{
			Rank: i + 1, ID: p.ID, Name: p.Name, Initial: initial(p.Name), WeekDone: week[p.ID], Me: me,
		}
		if premium {
			row.Level, row.Diamond = p.Level, p.Level >= 100
			row.Username = p.Username.String
			row.CanAdd = tab == "" && !me && p.Username.Valid && p.Username.String != ""
		}
		data.People = append(data.People, row)
	}

	a.render(w, "community.html", data)
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
