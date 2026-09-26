package handlers

import (
	"database/sql"
	"encoding/csv"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"version20/internal/payouts"
	"version20/internal/reports"
	"version20/internal/store"
)

// Раздел «Выводы» веб-админки: заявки с полным номером карты (только
// здесь он расшифровывается), «Выплачено» / «Отклонить», выгрузка для бухгалтера.

type AdminWithdrawalRow struct {
	store.Withdrawal
	Card string // полный номер (или маска, если ключ недоступен)
}

type AdminWithdrawalsData struct {
	Pending   []AdminWithdrawalRow
	Processed []store.Withdrawal
	Month     string
	CardKeyOK bool
}

func (a *App) handleAdminWithdrawals(w http.ResponseWriter, r *http.Request) {
	month := r.URL.Query().Get("month")
	if month == "" {
		month = payouts.MonthKey(a.now(), reports.MoscowLocation())
	}
	pending, err := a.Store.Withdrawals("pending", "")
	if err != nil {
		a.serverError(w, err)
		return
	}
	all, err := a.Store.Withdrawals("", month)
	if err != nil {
		a.serverError(w, err)
		return
	}
	data := AdminWithdrawalsData{Month: month, CardKeyOK: a.CardKey != nil}
	for _, p := range pending {
		row := AdminWithdrawalRow{Withdrawal: p, Card: "•••• " + p.CardLast4}
		if a.CardKey != nil {
			if full, err := a.CardKey.Decrypt(p.CardEnc); err == nil {
				row.Card = groupCard(full)
			}
		}
		data.Pending = append(data.Pending, row)
	}
	for _, p := range all {
		if p.Status != "pending" {
			data.Processed = append(data.Processed, p)
		}
	}
	a.renderAdmin(w, r, "admin_withdrawals.html", "Выводы", data)
}

// groupCard — "2200 1234 5678 9012".
func groupCard(d string) string {
	var b strings.Builder
	for i, c := range d {
		if i > 0 && i%4 == 0 {
			b.WriteByte(' ')
		}
		b.WriteRune(c)
	}
	return b.String()
}

func (a *App) finishWithdrawal(w http.ResponseWriter, r *http.Request, status string) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	r.ParseForm()
	reason := strings.TrimSpace(r.FormValue("reason"))
	if status == "rejected" && reason == "" {
		http.Redirect(w, r, flashURL("/admin/withdrawals", "err", "Укажите причину отказа", ""), http.StatusSeeOther)
		return
	}
	wd, err := a.Store.FinishWithdrawal(id, status, reason, a.now().UTC().Format(time.RFC3339))
	switch {
	case errors.Is(err, sql.ErrNoRows):
		http.NotFound(w, r)
		return
	case errors.Is(err, store.ErrWithdrawalProcessed):
		http.Redirect(w, r, flashURL("/admin/withdrawals", "err", "Заявка уже обработана", ""), http.StatusSeeOther)
		return
	case err != nil:
		a.serverError(w, err)
		return
	}
	if status == "paid" {
		a.notify(wd.TgID, fmt.Sprintf("Выплата %s отправлена на карту •••• %s", Rub(wd.Net), wd.CardLast4))
		http.Redirect(w, r, flashURL("/admin/withdrawals", "ok", "Отмечено как выплачено: "+wd.Name, ""), http.StatusSeeOther)
		return
	}
	a.notify(wd.TgID, fmt.Sprintf("Вывод %s отклонён: %s. Деньги вернулись в кошелёк.", Rub(wd.Gross), reason))
	http.Redirect(w, r, flashURL("/admin/withdrawals", "ok", "Заявка отклонена, деньги возвращены: "+wd.Name, ""), http.StatusSeeOther)
}

func (a *App) handleAdminWithdrawalPaid(w http.ResponseWriter, r *http.Request) {
	a.finishWithdrawal(w, r, "paid")
}

func (a *App) handleAdminWithdrawalReject(w http.ResponseWriter, r *http.Request) {
	a.finishWithdrawal(w, r, "rejected")
}

// handleAdminWithdrawalsCSV — выгрузка для бухгалтера за месяц (?month=2026-10).
func (a *App) handleAdminWithdrawalsCSV(w http.ResponseWriter, r *http.Request) {
	month := r.URL.Query().Get("month")
	if month == "" {
		month = payouts.MonthKey(a.now(), reports.MoscowLocation())
	}
	list, err := a.Store.Withdrawals("", month)
	if err != nil {
		a.serverError(w, err)
		return
	}
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="withdrawals-`+month+`.csv"`)
	w.Write([]byte{0xEF, 0xBB, 0xBF}) // BOM — чтобы Excel открыл кириллицу
	cw := csv.NewWriter(w)
	cw.Comma = ';'
	cw.Write([]string{"Имя", "@username", "Telegram ID", "Карта", "Начислено, ₽", "НДФЛ, ₽", "Выплачено, ₽", "Дата заявки", "Статус", "Дата обработки"})
	for _, x := range list {
		username := ""
		if x.Username.Valid && x.Username.String != "" {
			username = "@" + x.Username.String
		}
		cw.Write([]string{x.Name, username, x.TgID, "•••• " + x.CardLast4,
			strconv.Itoa(x.Gross), strconv.Itoa(x.Tax), strconv.Itoa(x.Net),
			x.CreatedAt[:min(10, len(x.CreatedAt))], withdrawalStatusLabels[x.Status], x.ProcessedAt.String})
	}
	cw.Flush()
}
