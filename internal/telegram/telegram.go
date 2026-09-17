// Package telegram — проверка initData и отправка сообщений через Bot API.
// Порт backend/lib/telegramAuth.js (verifyInitData) и backend/lib/telegram.js.
package telegram

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"sort"
	"strings"
)

type Identity struct {
	ID         string
	Username   string
	Name       string
	StartParam string // из ?start=CODE при запуске мини-аппа по реферальной ссылке
}

// VerifyInitData — алгоритм Telegram:
// https://core.telegram.org/bots/webapps#validating-data-received-via-the-mini-app
func VerifyInitData(initData, botToken string) (*Identity, bool) {
	if initData == "" || botToken == "" {
		return nil, false
	}
	values, err := url.ParseQuery(initData)
	if err != nil {
		return nil, false
	}
	hash := values.Get("hash")
	if hash == "" {
		return nil, false
	}
	startParam := values.Get("start_param")
	values.Del("hash")

	keys := make([]string, 0, len(values))
	for k := range values {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	pairs := make([]string, 0, len(keys))
	for _, k := range keys {
		pairs = append(pairs, k+"="+values.Get(k))
	}
	dataCheckString := strings.Join(pairs, "\n")

	secretKey := hmac.New(sha256.New, []byte("WebAppData"))
	secretKey.Write([]byte(botToken))
	secret := secretKey.Sum(nil)

	mac := hmac.New(sha256.New, secret)
	mac.Write([]byte(dataCheckString))
	computedHash := hex.EncodeToString(mac.Sum(nil))

	if computedHash != hash {
		return nil, false
	}

	userJSON := values.Get("user")
	if userJSON == "" {
		return nil, false
	}
	var u struct {
		ID        int64  `json:"id"`
		Username  string `json:"username"`
		FirstName string `json:"first_name"`
	}
	if err := json.Unmarshal([]byte(userJSON), &u); err != nil {
		return nil, false
	}
	name := u.FirstName
	if name == "" {
		name = "Друг"
	}
	return &Identity{ID: fmt.Sprintf("%d", u.ID), Username: u.Username, Name: name, StartParam: startParam}, true
}

var warnedOnce = false

// SendMessage — тихо ничего не делает и логирует один раз, если BOT_TOKEN
// не задан, чтобы сервер не падал из-за отсутствия токена в деве.
func SendMessage(botToken, tgID, text string) {
	if botToken == "" {
		if !warnedOnce {
			log.Println("[telegram] BOT_TOKEN не задан — push-уведомления и сообщения бота отключены.")
			warnedOnce = true
		}
		return
	}

	body, _ := json.Marshal(map[string]string{
		"chat_id":    tgID,
		"text":       text,
		"parse_mode": "HTML",
	})
	resp, err := http.Post(
		fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", botToken),
		"application/json", bytes.NewReader(body),
	)
	if err != nil {
		log.Println("[telegram] sendMessage error:", err)
		return
	}
	defer resp.Body.Close()
}
