package handlers

import "net/http"

type WalletData struct {
	Code            string
	Balance         int
	PayingReferrals int
	History         []WalletHistoryItem
	Message         string
}

const withdrawSoonMsg = "Вывод средств скоро появится — сейчас баланс копится в приложении."

type WalletHistoryItem struct {
	Amount    int
	Note      string
	CreatedAt string
}

func (a *App) handleWalletShow(w http.ResponseWriter, r *http.Request) {
	user := userFromCtx(r)

	balance, err := a.Store.WalletBalance(user.ID)
	if err != nil {
		a.serverError(w, err)
		return
	}
	paying, err := a.Store.PayingReferralsCount(user.ID)
	if err != nil {
		a.serverError(w, err)
		return
	}
	history, err := a.Store.WalletHistory(user.ID)
	if err != nil {
		a.serverError(w, err)
		return
	}

	items := make([]WalletHistoryItem, 0, len(history))
	for _, h := range history {
		items = append(items, WalletHistoryItem{Amount: h.Amount, Note: h.Note.String, CreatedAt: h.CreatedAt})
	}

	a.render(w, "wallet.html", WalletData{
		Code: user.ReferralCode.String, Balance: balance, PayingReferrals: paying, History: items,
		Message: map[string]string{"soon": withdrawSoonMsg}[r.URL.Query().Get("msg")],
	})
}

func (a *App) handleWalletWithdraw(w http.ResponseWriter, r *http.Request) {
	http.Redirect(w, r, "/wallet?msg=soon", http.StatusSeeOther)
}
