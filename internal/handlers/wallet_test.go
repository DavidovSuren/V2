package handlers

import (
	"encoding/base64"
	"net/url"
	"strings"
	"testing"
	"time"

	"version20/internal/cardcrypt"
	"version20/internal/reports"
)

const testCard = "4111 1111 1111 1111" // проходит проверку Луна

func withCardKey(t *testing.T, a *App) {
	t.Helper()
	k, err := cardcrypt.ParseKey(base64.StdEncoding.EncodeToString([]byte("0123456789abcdef0123456789abcdef")))
	if err != nil {
		t.Fatal(err)
	}
	a.CardKey = k
}

func moscowDay(day int) func() time.Time {
	return func() time.Time { return time.Date(2026, 10, day, 12, 0, 0, 0, reports.MoscowLocation()) }
}

func (a *App) giveBalance(t *testing.T, userID int64, amount int) {
	t.Helper()
	a.exec(t, `INSERT INTO wallet_transactions (user_id, amount, note, created_at, kind) VALUES ($1, $2, 'Партнёрский доход: тест', '2026-10-01T00:00:00Z', 'income')`, userID, amount)
}

// Пользователь с балансом. Регистрация раньше «сегодня» теста, а доступ к
// кошельку от подписки не зависит.
func walletUser(t *testing.T, a *App, tg string, balance int) int64 {
	t.Helper()
	u := a.newPlayer(t, tg, "")
	a.giveBalance(t, u.ID, balance)
	return u.ID
}

func TestWithdrawOnlyOn15th(t *testing.T) {
	a := newDBApp(t)
	withCardKey(t, a)
	id := walletUser(t, a, "1", 5772)

	a.Now = moscowDay(14)
	mustContain(t, a.get(t, "1", "/wallet").Body, "Вывод 15 октября", "5 772 ₽")
	r := a.post(t, "1", "/wallet/withdraw", url.Values{"card": {testCard}})
	mustContain(t, r.Body, "Вывод доступен 15 октября")
	if n := a.count(t, "SELECT COUNT(*) FROM withdrawals WHERE user_id=$1", id); n != 0 {
		t.Fatalf("заявка создана 14-го: %d", n)
	}

	a.Now = moscowDay(16)
	mustContain(t, a.get(t, "1", "/wallet").Body, "Вывод 15 ноября")
}

func TestWithdrawOn15th(t *testing.T) {
	a := newDBApp(t)
	withCardKey(t, a)
	msgs := a.captureNotify()
	a.AdminTgID = "999"
	id := walletUser(t, a, "1", 5772)
	a.Now = moscowDay(15)

	mustContain(t, a.get(t, "1", "/wallet").Body, `href="/wallet/withdraw"`)
	form := a.get(t, "1", "/wallet/withdraw").Body
	mustContain(t, form, "Сегодня 15-е — день вывода", "5 772 ₽", "−750 ₽", "Вывести 5 022 ₽", `inputmode="numeric"`)
	// В форме только номер карты — ни ФИО, ни ИНН, ни галочек.
	formPart := form[strings.Index(form, `id="withdraw-form"`):]
	formPart = formPart[:strings.Index(formPart, "</form>")]
	if n := strings.Count(formPart, `name="`); n != 1 || !strings.Contains(formPart, `name="card"`) {
		t.Errorf("полей в форме вывода: %d", n)
	}

	r := a.post(t, "1", "/wallet/withdraw", url.Values{"card": {testCard}})
	if r.Location != "/wallet?withdraw=ok" {
		t.Fatalf("вывод: %d %q", r.Code, r.Location)
	}
	var gross, tax, net, pct int
	var last4, enc, status, month string
	a.Store.DB.QueryRow(`SELECT gross, tax, net, tax_pct, card_last4, card_enc, status, month_key FROM withdrawals WHERE user_id=$1`, id).
		Scan(&gross, &tax, &net, &pct, &last4, &enc, &status, &month)
	if gross != 5772 || tax != 750 || net != 5022 || pct != 13 || last4 != "1111" || status != "pending" || month != "2026-10" {
		t.Errorf("заявка: %d %d %d %d%% %s %s %s", gross, tax, net, pct, last4, status, month)
	}
	if strings.Contains(enc, "4111") || enc == "" {
		t.Errorf("карта хранится открытой: %q", enc)
	}
	if bal, _ := a.Store.WalletBalance(id); bal != 0 {
		t.Errorf("баланс после вывода: %d", bal)
	}
	if len(*msgs) != 1 || !strings.HasPrefix((*msgs)[0], "999: Новая заявка на вывод: User 1") {
		t.Errorf("сообщение админу: %v", *msgs)
	}
	wallet := a.get(t, "1", "/wallet?withdraw=ok").Body
	mustContain(t, wallet, "Заявка на вывод принята", "Вывод на карту •••• 1111", "в обработке", "выведено 5 772 ₽")

	// Второй вывод в том же месяце — отказ, даже если деньги снова есть.
	a.giveBalance(t, id, 1000)
	mustContain(t, a.post(t, "1", "/wallet/withdraw", url.Values{"card": {testCard}}).Body, "В этом месяце вывод уже был")
	if n := a.count(t, "SELECT COUNT(*) FROM withdrawals WHERE user_id=$1", id); n != 1 {
		t.Errorf("заявок: %d", n)
	}

	// Через месяц карта уже сохранена: поле заполнено маской, вводить не нужно.
	a.Now = func() time.Time { return time.Date(2026, 11, 15, 12, 0, 0, 0, reports.MoscowLocation()) }
	mustContain(t, a.get(t, "1", "/wallet/withdraw").Body, `value="•••• •••• •••• 1111"`)
	if r := a.post(t, "1", "/wallet/withdraw", url.Values{"card": {"•••• •••• •••• 1111"}}); r.Location != "/wallet?withdraw=ok" {
		t.Fatalf("вывод сохранённой картой: %d %q %s", r.Code, r.Location, r.Body)
	}
}

func TestWithdrawRefusals(t *testing.T) {
	a := newDBApp(t)
	withCardKey(t, a)
	a.Now = moscowDay(15)

	walletUser(t, a, "1", 499)
	mustContain(t, a.post(t, "1", "/wallet/withdraw", url.Values{"card": {testCard}}).Body, "Минимальная сумма вывода — 500 ₽")

	id := walletUser(t, a, "2", 800)
	r := a.post(t, "2", "/wallet/withdraw", url.Values{"card": {"4111 1111 1111 1112"}})
	mustContain(t, r.Body, "Проверь номер карты", `class="field-error"`)
	mustContain(t, a.post(t, "2", "/wallet/withdraw", url.Values{"card": {""}}).Body, "Введи номер карты")
	if n := a.count(t, "SELECT COUNT(*) FROM withdrawals WHERE user_id=$1", id); n != 0 {
		t.Errorf("заявка с неверной картой: %d", n)
	}

	a.CardKey = nil
	mustContain(t, a.post(t, "2", "/wallet/withdraw", url.Values{"card": {testCard}}).Body, "Вывод временно недоступен")
}

func TestAdminProcessesWithdrawals(t *testing.T) {
	a := withPanels(newDBApp(t))
	withCardKey(t, a)
	msgs := a.captureNotify()
	a.Now = moscowDay(15)
	id := walletUser(t, a, "1", 1000)
	walletUser(t, a, "2", 2000)
	a.post(t, "1", "/wallet/withdraw", url.Values{"card": {testCard}})
	a.post(t, "2", "/wallet/withdraw", url.Values{"card": {"5555 5555 5555 4444"}})

	// Сессия Mini App к разделу не подходит.
	if r := a.get(t, "1", "/admin/withdrawals"); r.Location != "/admin/login" {
		t.Errorf("не админ открыл выводы: %d %q", r.Code, r.Location)
	}

	page := a.adminGet(t, "/admin/withdrawals").Body
	mustContain(t, page, "4111 1111 1111 1111", "5555 5555 5555 4444", "870 ₽", "1 740 ₽")

	var w1, w2 int64
	a.Store.DB.QueryRow(`SELECT id FROM withdrawals WHERE user_id=$1`, id).Scan(&w1)
	a.Store.DB.QueryRow(`SELECT id FROM withdrawals WHERE user_id<>$1`, id).Scan(&w2)

	if r := a.adminPost(t, "/admin/withdrawals/"+itoa(int(w1))+"/reject", url.Values{}); !strings.Contains(r.Location, "err=") {
		t.Errorf("отказ без причины: %q", r.Location)
	}
	a.adminPost(t, "/admin/withdrawals/"+itoa(int(w1))+"/reject", url.Values{"reason": {"карта заблокирована"}})
	if bal, _ := a.Store.WalletBalance(id); bal != 1000 {
		t.Errorf("деньги не вернулись: %d", bal)
	}
	a.adminPost(t, "/admin/withdrawals/"+itoa(int(w2))+"/paid", nil)
	if r := a.adminPost(t, "/admin/withdrawals/"+itoa(int(w2))+"/paid", nil); !strings.Contains(r.Location, "err=") {
		t.Errorf("повторная обработка: %q", r.Location)
	}

	var s1, s2 string
	a.Store.DB.QueryRow(`SELECT status FROM withdrawals WHERE id=$1`, w1).Scan(&s1)
	a.Store.DB.QueryRow(`SELECT status FROM withdrawals WHERE id=$1`, w2).Scan(&s2)
	if s1 != "rejected" || s2 != "paid" {
		t.Errorf("статусы: %s %s", s1, s2)
	}
	joined := strings.Join(*msgs, "\n")
	mustContain(t, joined, "1: Вывод 1 000 ₽ отклонён: карта заблокирована", "2: Выплата 1 740 ₽ отправлена на карту •••• 4444")

	// Отклонённая заявка не мешает подать новую в том же месяце.
	if r := a.post(t, "1", "/wallet/withdraw", url.Values{"card": {testCard}}); r.Location != "/wallet?withdraw=ok" {
		t.Errorf("повторная заявка после отказа: %q", r.Location)
	}

	csv := a.adminGet(t, "/admin/withdrawals.csv?month=2026-10")
	mustContain(t, csv.Body, "Имя;@username;Telegram ID", "User 2;;2;•••• 4444;2000;260;1740")
	mustNotContain(t, csv.Body, "5555 5555")
	mustContain(t, a.adminGet(t, "/admin").Body, "заявок на вывод")
}

func TestWalletRateAndInvitees(t *testing.T) {
	a := newDBApp(t)
	ref := a.newPlayer(t, "100", "")
	mustContain(t, a.get(t, "100", "/wallet").Body, "Нет подписки — доход не начисляется")

	a.post(t, "100", "/subscribe", url.Values{"tier": {"premium888"}})
	a.newPlayer(t, "200", ref.ReferralCode.String)
	a.post(t, "200", "/subscribe", url.Values{"tier": {"plus369"}})
	a.newPlayer(t, "201", ref.ReferralCode.String)
	a.newPlayer(t, "202", ref.ReferralCode.String)
	a.exec(t, "UPDATE users SET created_at='2020-01-01T00:00:00Z' WHERE tg_id='202'")

	a.BotUsername, a.MiniAppShortName = "version20bot", "app"
	w := a.get(t, "100", "/wallet").Body
	mustContain(t, w, "Premium · 20%", "ещё 9 оплативших до 50%",
		"User 200", "Plus · ", "+74 ₽",
		"User 201", "Пробный период · осталось 3 дн.", "ждём оплату",
		"User 202", "Пробный закончился", "не оплатил",
		"https://t.me/version20bot/app?startapp="+ref.ReferralCode.String, "https://t.me/share/url?", "data-tg-link")
}
