package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"version20/internal/telegram"
)

type fakeBot struct {
	invoices []telegram.Invoice
	answers  []string // "<id>:<ok>:<msg>"
	buttons  []string // "<chat>|<url>"
	failLink bool
}

func (f *fakeBot) CreateInvoiceLink(inv telegram.Invoice) (string, error) {
	f.invoices = append(f.invoices, inv)
	if f.failLink {
		return "", fmt.Errorf("provider error")
	}
	return "https://t.me/$invoice123", nil
}

func (f *fakeBot) AnswerPreCheckoutQuery(id string, ok bool, msg string) error {
	f.answers = append(f.answers, fmt.Sprintf("%s:%v:%s", id, ok, msg))
	return nil
}

func (f *fakeBot) SendMessageWithWebAppButton(chatID, text, button, u string) error {
	f.buttons = append(f.buttons, chatID+"|"+u)
	return nil
}

// withRealPayments — как в проде: без тест-режима, с платёжным токеном.
func withRealPayments(a *App) *fakeBot {
	bot := &fakeBot{}
	a.Bot = bot
	a.Payments = PaymentConfig{ProviderToken: "provider-token"}
	a.PublicURL = "https://v2.example"
	return bot
}

func (a *App) webhook(t *testing.T, secret string, update any) resp {
	t.Helper()
	body, _ := json.Marshal(update)
	return a.doWithHeader(t, "POST", "/telegram/webhook", string(body), map[string]string{
		"Content-Type": "application/json", "X-Telegram-Bot-Api-Secret-Token": secret,
	})
}

func TestWebhookRequiresSecret(t *testing.T) {
	a := newTestApp(t)
	for name, secret := range map[string]string{"без секрета": "", "чужой секрет": "0123456789abcdef0123456789abcdef"} {
		r := a.doWithHeader(t, "POST", "/telegram/webhook", `{}`, map[string]string{"X-Telegram-Bot-Api-Secret-Token": secret})
		if r.Code != http.StatusUnauthorized {
			t.Errorf("%s: %d", name, r.Code)
		}
	}
	a.BotToken = ""
	r := a.doWithHeader(t, "POST", "/telegram/webhook", `{}`, map[string]string{"X-Telegram-Bot-Api-Secret-Token": WebhookSecret("")})
	if r.Code != http.StatusUnauthorized {
		t.Errorf("без BOT_TOKEN: %d", r.Code)
	}
	if s := WebhookSecret(testBotToken); len(s) != 32 {
		t.Errorf("секрет: %q", s)
	}
}

func TestSubscribeWithoutPaymentsDoesNotActivate(t *testing.T) {
	a := newDBApp(t)
	a.Payments = PaymentConfig{} // ни токена, ни тест-режима
	u := a.newPlayer(t, "1", "")
	r := a.post(t, "1", "/subscribe", url.Values{"tier": {"premium888"}})
	mustContain(t, r.Body, "Оплата временно недоступна")
	if got := a.user(t, "1"); got.SubscriptionTier != "free" || len(a.payments(t, u.ID)) != 0 {
		t.Errorf("подписка активирована без оплаты: %+v", got)
	}
}

func TestSubscribeCreatesInvoice(t *testing.T) {
	a := newDBApp(t)
	bot := withRealPayments(a)
	u := a.newPlayer(t, "1", "")

	r := a.post(t, "1", "/subscribe", url.Values{"tier": {"plus369"}})
	mustContain(t, r.Body, `data-invoice-link="https://t.me/$invoice123"`, "/static/pay.js")
	if len(bot.invoices) != 1 {
		t.Fatalf("инвойсов: %d", len(bot.invoices))
	}
	inv := bot.invoices[0]
	if inv.Payload != fmt.Sprintf("sub:%d:plus369", u.ID) || inv.Currency != "RUB" || inv.ProviderToken != "provider-token" ||
		len(inv.Prices) != 1 || inv.Prices[0].Amount != 36900 || inv.Prices[0].Label != "Version 2.0 Plus, 1 месяц" ||
		!inv.NeedEmail || !inv.SendEmailToProvider || !strings.Contains(inv.ProviderData, `"value":"369.00"`) {
		t.Errorf("инвойс: %+v", inv)
	}
	if got := a.user(t, "1"); got.SubscriptionTier != "free" {
		t.Error("подписка активирована до оплаты")
	}

	// Ошибка провайдера — понятная страница, без активации.
	bot.failLink = true
	mustContain(t, a.post(t, "1", "/subscribe", url.Values{"tier": {"plus369"}}).Body, "Оплата временно недоступна")

	// Telegram Stars: цена в звёздах, без токена провайдера и чека.
	bot.failLink = false
	a.Payments = PaymentConfig{Currency: "XTR", StarsPrices: map[string]int{"plus369": 250, "premium888": 600}}
	a.post(t, "1", "/subscribe", url.Values{"tier": {"premium888"}})
	stars := bot.invoices[len(bot.invoices)-1]
	if stars.Currency != "XTR" || stars.Prices[0].Amount != 600 || stars.ProviderToken != "" || stars.ProviderData != "" {
		t.Errorf("инвойс в звёздах: %+v", stars)
	}
}

func TestPreCheckoutValidatesAmount(t *testing.T) {
	a := newDBApp(t)
	bot := withRealPayments(a)
	u := a.newPlayer(t, "1", "")
	secret := WebhookSecret(testBotToken)

	pre := func(id, payload string, amount int) {
		a.webhook(t, secret, map[string]any{"pre_checkout_query": map[string]any{
			"id": id, "currency": "RUB", "total_amount": amount, "invoice_payload": payload,
		}})
	}
	pre("ok", fmt.Sprintf("sub:%d:premium888", u.ID), 88800)
	pre("sum", fmt.Sprintf("sub:%d:premium888", u.ID), 100)
	pre("user", "sub:999999:plus369", 36900)
	pre("junk", "hello", 36900)

	want := []string{"ok:true:", "sum:false:Сумма", "user:false:Пользователь", "junk:false:Не удалось"}
	if len(bot.answers) != len(want) {
		t.Fatalf("ответы: %v", bot.answers)
	}
	for i, w := range want {
		if !strings.HasPrefix(bot.answers[i], w) {
			t.Errorf("ответ %d: %q, want %q…", i, bot.answers[i], w)
		}
	}
}

func TestSuccessfulPaymentIsIdempotent(t *testing.T) {
	a := newDBApp(t)
	withRealPayments(a)
	a.Payments.TestMode = true
	ref := a.newPlayer(t, "100", "")
	a.post(t, "100", "/subscribe", url.Values{"tier": {"premium888"}})
	a.Payments.TestMode = false
	u := a.newPlayer(t, "200", ref.ReferralCode.String)

	update := map[string]any{"message": map[string]any{
		"chat": map[string]any{"id": 200},
		"successful_payment": map[string]any{
			"currency": "RUB", "total_amount": 36900, "invoice_payload": fmt.Sprintf("sub:%d:plus369", u.ID),
			"telegram_payment_charge_id": "tg_charge_42",
		},
	}}
	for i := 0; i < 2; i++ {
		if r := a.webhook(t, WebhookSecret(testBotToken), update); r.Code != 200 {
			t.Fatalf("вебхук: %d", r.Code)
		}
	}
	if got := a.user(t, "200"); got.SubscriptionTier != "plus369" {
		t.Errorf("подписка не активирована: %s", got.SubscriptionTier)
	}
	if n := len(a.payments(t, u.ID)); n != 1 {
		t.Errorf("оплат: %d, want 1", n)
	}
	if bal, _ := a.Store.WalletBalance(ref.ID); bal != 74 {
		t.Errorf("начисление: %d, want 74", bal)
	}
}

func TestStartCommandOpensMiniApp(t *testing.T) {
	a := newTestApp(t)
	bot := withRealPayments(a)
	secret := WebhookSecret(testBotToken)
	start := func(text string) {
		a.webhook(t, secret, map[string]any{"message": map[string]any{"chat": map[string]any{"id": 77}, "text": text}})
	}
	start("/start K7QX2PD")
	start("/start")
	start("/start <script>")

	want := []string{"77|https://v2.example/?ref=K7QX2PD", "77|https://v2.example/", "77|https://v2.example/"}
	if strings.Join(bot.buttons, ",") != strings.Join(want, ",") {
		t.Errorf("кнопки: %v", bot.buttons)
	}
}

func TestRefQueryParam(t *testing.T) {
	a := newTestApp(t)
	r := a.do(t, "GET", "/?ref=K7QX2PD", nil, "")
	if c := cookieByName(r.Cookies, "v2_ref"); c == nil || c.Value != "K7QX2PD" {
		t.Errorf("cookie: %v", c)
	}
	mustContain(t, r.Body, `name="refCode" value="K7QX2PD"`)
	if r := a.do(t, "GET", "/?ref=%3Cx%3E", nil, ""); cookieByName(r.Cookies, "v2_ref") != nil {
		t.Error("небезопасный код попал в cookie")
	}
}
