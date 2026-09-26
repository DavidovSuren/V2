package handlers

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"

	"version20/internal/referrals"
	"version20/internal/telegram"
)

// BotAPI — то, что нужно от Bot API для оплаты; в тестах подменяется.
type BotAPI interface {
	CreateInvoiceLink(inv telegram.Invoice) (string, error)
	AnswerPreCheckoutQuery(queryID string, ok bool, errorMessage string) error
	SendMessageWithWebAppButton(chatID, text, buttonText, webAppURL string) error
}

// PaymentConfig — переменные окружения оплаты (см. .env.example).
type PaymentConfig struct {
	ProviderToken string         // PAYMENT_PROVIDER_TOKEN — ЮKassa из @BotFather → Payments
	Currency      string         // PAYMENT_CURRENCY — RUB (по умолчанию) или XTR (Telegram Stars)
	StarsPrices   map[string]int // STARS_PRICE_PLUS / STARS_PRICE_PREMIUM, если валюта XTR
	TestMode      bool           // PAYMENTS_TEST_MODE — «Оформить» активирует сразу (только локально/в тестах)
}

func (c PaymentConfig) currency() string {
	if c.Currency == "" {
		return "RUB"
	}
	return c.Currency
}

// amount — сумма инвойса в минимальных единицах валюты: копейки для RUB,
// звёзды для XTR. 0 — оплата в этой валюте не настроена.
func (c PaymentConfig) amount(tier string) int {
	if c.currency() == "XTR" {
		return c.StarsPrices[tier]
	}
	return referrals.Prices[tier] * 100
}

func (a *App) paymentsAvailable() bool {
	if a.Bot == nil {
		return false
	}
	if a.Payments.currency() == "XTR" {
		return true
	}
	return a.Payments.ProviderToken != ""
}

// WebhookSecret — secret_token вебхука: первые 32 символа hex(sha256(BOT_TOKEN)).
func WebhookSecret(botToken string) string {
	sum := sha256.Sum256([]byte(botToken))
	return hex.EncodeToString(sum[:])[:32]
}

func randomHex(n int) string {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

type PayPageData struct {
	Link        string
	TierName    string
	Unavailable bool
}

// handleSubscribe — «Оформить». Подписка активируется только после
// подтверждённой оплаты (successful_payment в вебхуке), кроме тест-режима.
func (a *App) handleSubscribe(w http.ResponseWriter, r *http.Request) {
	user := userFromCtx(r)
	r.ParseForm()
	tier := r.FormValue("tier")
	if _, ok := referrals.Prices[tier]; !ok {
		http.Redirect(w, r, "/plans", http.StatusSeeOther)
		return
	}

	if a.Payments.TestMode {
		if err := a.activateSubscription(user.ID, tier, "test-"+randomHex(8)); err != nil {
			a.serverError(w, err)
			return
		}
		http.Redirect(w, r, "/plans?paid=1", http.StatusSeeOther)
		return
	}

	if !a.paymentsAvailable() || a.Payments.amount(tier) <= 0 {
		a.render(w, "pay.html", PayPageData{Unavailable: true})
		return
	}

	link, err := a.Bot.CreateInvoiceLink(a.invoiceFor(user.ID, tier))
	if err != nil {
		log.Println("[payments] createInvoiceLink:", err)
		a.render(w, "pay.html", PayPageData{Unavailable: true})
		return
	}
	a.render(w, "pay.html", PayPageData{Link: link, TierName: referrals.TierNames[tier]})
}

func (a *App) invoiceFor(userID int64, tier string) telegram.Invoice {
	name := "Version 2.0 " + referrals.TierNames[tier] + ", 1 месяц"
	inv := telegram.Invoice{
		Title:       "Version 2.0 " + referrals.TierNames[tier],
		Description: "Подписка на 1 месяц: все задания, отчёты и дневник",
		Payload:     fmt.Sprintf("sub:%d:%s", userID, tier),
		Currency:    a.Payments.currency(),
		Prices:      []telegram.LabeledPrice{{Label: name, Amount: a.Payments.amount(tier)}},
	}
	if inv.Currency == "RUB" {
		inv.ProviderToken = a.Payments.ProviderToken
		inv.NeedEmail = true
		inv.SendEmailToProvider = true
		// Чек для ЮKassa по 54-ФЗ. vat_code 1 — «без НДС»: уточни у бухгалтера.
		receipt, _ := json.Marshal(map[string]any{"receipt": map[string]any{"items": []map[string]any{{
			"description": name,
			"quantity":    "1.00",
			"amount":      map[string]string{"value": fmt.Sprintf("%d.00", referrals.Prices[tier]), "currency": "RUB"},
			"vat_code":    1,
		}}}})
		inv.ProviderData = string(receipt)
	}
	return inv
}

// parsePayload — "sub:<userID>:<tier>".
func parsePayload(payload string) (int64, string, bool) {
	parts := strings.Split(payload, ":")
	if len(parts) != 3 || parts[0] != "sub" {
		return 0, "", false
	}
	id, err := strconv.ParseInt(parts[1], 10, 64)
	if _, ok := referrals.Prices[parts[2]]; err != nil || !ok {
		return 0, "", false
	}
	return id, parts[2], true
}

type tgUpdate struct {
	PreCheckoutQuery *struct {
		ID             string `json:"id"`
		Currency       string `json:"currency"`
		TotalAmount    int    `json:"total_amount"`
		InvoicePayload string `json:"invoice_payload"`
	} `json:"pre_checkout_query"`
	Message *struct {
		Chat struct {
			ID int64 `json:"id"`
		} `json:"chat"`
		Text              string `json:"text"`
		SuccessfulPayment *struct {
			Currency                string `json:"currency"`
			TotalAmount             int    `json:"total_amount"`
			InvoicePayload          string `json:"invoice_payload"`
			TelegramPaymentChargeID string `json:"telegram_payment_charge_id"`
		} `json:"successful_payment"`
	} `json:"message"`
}

// handleTelegramWebhook — обновления от Telegram: проверка перед оплатой,
// успешная оплата и /start с реферальным кодом.
func (a *App) handleTelegramWebhook(w http.ResponseWriter, r *http.Request) {
	got := r.Header.Get("X-Telegram-Bot-Api-Secret-Token")
	if a.BotToken == "" || subtle.ConstantTimeCompare([]byte(got), []byte(WebhookSecret(a.BotToken))) != 1 {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	var upd tgUpdate
	if err := json.Unmarshal(body, &upd); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	switch {
	case upd.PreCheckoutQuery != nil:
		q := upd.PreCheckoutQuery
		ok, msg := a.checkPreCheckout(q.InvoicePayload, q.Currency, q.TotalAmount)
		if a.Bot != nil {
			if err := a.Bot.AnswerPreCheckoutQuery(q.ID, ok, msg); err != nil {
				log.Println("[payments] answerPreCheckoutQuery:", err)
			}
		}
	case upd.Message != nil && upd.Message.SuccessfulPayment != nil:
		p := upd.Message.SuccessfulPayment
		userID, tier, ok := parsePayload(p.InvoicePayload)
		if !ok {
			log.Println("[payments] successful_payment с неизвестным payload:", p.InvoicePayload)
			break
		}
		if err := a.activateSubscription(userID, tier, p.TelegramPaymentChargeID); err != nil {
			// 500 — Telegram повторит доставку; activateSubscription идемпотентна.
			a.serverError(w, err)
			return
		}
	case upd.Message != nil && strings.HasPrefix(upd.Message.Text, "/start"):
		a.answerStart(upd.Message.Chat.ID, strings.TrimSpace(strings.TrimPrefix(upd.Message.Text, "/start")))
	}
	w.WriteHeader(http.StatusOK)
}

func (a *App) checkPreCheckout(payload, currency string, total int) (bool, string) {
	userID, tier, ok := parsePayload(payload)
	if !ok {
		return false, "Не удалось распознать тариф. Открой оплату заново из приложения."
	}
	u, err := a.Store.GetUserByID(userID)
	if err != nil || u == nil {
		return false, "Пользователь не найден. Открой приложение и попробуй ещё раз."
	}
	if currency != a.Payments.currency() || total != a.Payments.amount(tier) {
		return false, "Сумма не совпадает с ценой тарифа. Открой оплату заново из приложения."
	}
	return true, ""
}

// answerStart — /start или /start КОД: кнопка, открывающая Mini App (с
// реферальным кодом, если он есть — так работает ссылка t.me/<бот>?start=КОД).
func (a *App) answerStart(chatID int64, code string) {
	if a.Bot == nil || a.PublicURL == "" {
		return
	}
	url := strings.TrimRight(a.PublicURL, "/") + "/"
	if code != "" && isSafeCode(code) {
		url += "?ref=" + code
	}
	text := "Привет. Version 2.0 — одно действие в день, 365 дней, новая версия себя. Первые 3 дня бесплатно."
	if err := a.Bot.SendMessageWithWebAppButton(strconv.FormatInt(chatID, 10), text, "Открыть Version 2.0", url); err != nil {
		log.Println("[bot] /start:", err)
	}
}

// isSafeCode — реферальный код из ссылки: только буквы, цифры, - и _.
func isSafeCode(s string) bool {
	if len(s) == 0 || len(s) > 32 {
		return false
	}
	for _, c := range s {
		if !(c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '-' || c == '_') {
			return false
		}
	}
	return true
}
