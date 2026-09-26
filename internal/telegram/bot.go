package telegram

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"
)

// Bot — клиент Bot API для оплаты и кнопок Mini App. BaseURL подменяется в
// тестах (по умолчанию https://api.telegram.org).
type Bot struct {
	Token   string
	BaseURL string
	HTTP    *http.Client
}

func NewBot(token string) *Bot {
	return &Bot{Token: token, BaseURL: "https://api.telegram.org", HTTP: &http.Client{Timeout: 15 * time.Second}}
}

func (b *Bot) call(method string, params any, result any) error {
	body, err := json.Marshal(params)
	if err != nil {
		return err
	}
	resp, err := b.HTTP.Post(fmt.Sprintf("%s/bot%s/%s", b.BaseURL, b.Token, method), "application/json", bytes.NewReader(body))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	var out struct {
		OK          bool            `json:"ok"`
		Result      json.RawMessage `json:"result"`
		Description string          `json:"description"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return fmt.Errorf("%s: %w", method, err)
	}
	if !out.OK {
		return errors.New(method + ": " + out.Description)
	}
	if result != nil {
		return json.Unmarshal(out.Result, result)
	}
	return nil
}

// GetMe возвращает @username бота (без @).
func (b *Bot) GetMe() (string, error) {
	var me struct {
		Username string `json:"username"`
	}
	err := b.call("getMe", struct{}{}, &me)
	return me.Username, err
}

type LabeledPrice struct {
	Label  string `json:"label"`
	Amount int    `json:"amount"` // в копейках для RUB, в звёздах для XTR
}

// Invoice — параметры createInvoiceLink.
type Invoice struct {
	Title               string         `json:"title"`
	Description         string         `json:"description"`
	Payload             string         `json:"payload"`
	ProviderToken       string         `json:"provider_token,omitempty"` // пусто для Telegram Stars
	Currency            string         `json:"currency"`
	Prices              []LabeledPrice `json:"prices"`
	NeedEmail           bool           `json:"need_email,omitempty"`
	SendEmailToProvider bool           `json:"send_email_to_provider,omitempty"`
	ProviderData        string         `json:"provider_data,omitempty"`
}

func (b *Bot) CreateInvoiceLink(inv Invoice) (string, error) {
	var link string
	err := b.call("createInvoiceLink", inv, &link)
	return link, err
}

func (b *Bot) AnswerPreCheckoutQuery(queryID string, ok bool, errorMessage string) error {
	params := map[string]any{"pre_checkout_query_id": queryID, "ok": ok}
	if !ok {
		params["error_message"] = errorMessage
	}
	return b.call("answerPreCheckoutQuery", params, nil)
}

// SetWebhook — url вебхука и secret_token, который Telegram присылает в
// заголовке X-Telegram-Bot-Api-Secret-Token.
func (b *Bot) SetWebhook(url, secret string) error {
	return b.call("setWebhook", map[string]any{
		"url":             url,
		"secret_token":    secret,
		"allowed_updates": []string{"message", "pre_checkout_query"},
	}, nil)
}

// SendMessageWithWebAppButton — сообщение с кнопкой, открывающей Mini App.
func (b *Bot) SendMessageWithWebAppButton(chatID, text, buttonText, webAppURL string) error {
	return b.call("sendMessage", map[string]any{
		"chat_id": chatID,
		"text":    text,
		"reply_markup": map[string]any{
			"inline_keyboard": [][]map[string]any{{
				{"text": buttonText, "web_app": map[string]string{"url": webAppURL}},
			}},
		},
	}, nil)
}
