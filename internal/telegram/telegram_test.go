package telegram

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"net/url"
	"sort"
	"strings"
	"testing"
)

const testToken = "123456:TEST-token"

// signInitData собирает initData так же, как это делает Telegram.
func signInitData(t *testing.T, token string, fields map[string]string) string {
	t.Helper()
	keys := make([]string, 0, len(fields))
	for k := range fields {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	pairs := make([]string, 0, len(keys))
	for _, k := range keys {
		pairs = append(pairs, k+"="+fields[k])
	}

	secret := hmac.New(sha256.New, []byte("WebAppData"))
	secret.Write([]byte(token))
	mac := hmac.New(sha256.New, secret.Sum(nil))
	mac.Write([]byte(strings.Join(pairs, "\n")))

	v := url.Values{}
	for k, val := range fields {
		v.Set(k, val)
	}
	v.Set("hash", hex.EncodeToString(mac.Sum(nil)))
	return v.Encode()
}

func TestVerifyInitDataValid(t *testing.T) {
	initData := signInitData(t, testToken, map[string]string{
		"auth_date":   "1700000000",
		"query_id":    "AAH",
		"start_param": "REFCODE",
		"user":        `{"id":42,"first_name":"Анна","username":"anna_v2"}`,
	})
	id, ok := VerifyInitData(initData, testToken)
	if !ok {
		t.Fatal("валидная подпись отвергнута")
	}
	if id.ID != "42" || id.Username != "anna_v2" || id.Name != "Анна" || id.StartParam != "REFCODE" {
		t.Errorf("identity: %+v", id)
	}
}

func TestVerifyInitDataDefaultName(t *testing.T) {
	initData := signInitData(t, testToken, map[string]string{
		"auth_date": "1700000000",
		"user":      `{"id":7}`,
	})
	id, ok := VerifyInitData(initData, testToken)
	if !ok {
		t.Fatal("отвергнуто")
	}
	if id.Name != "Друг" || id.Username != "" {
		t.Errorf("identity: %+v", id)
	}
}

func TestVerifyInitDataRejects(t *testing.T) {
	valid := signInitData(t, testToken, map[string]string{
		"auth_date": "1700000000",
		"user":      `{"id":42,"first_name":"A"}`,
	})
	noUser := signInitData(t, testToken, map[string]string{"auth_date": "1700000000"})
	badJSON := signInitData(t, testToken, map[string]string{"auth_date": "1", "user": "{not json"})

	cases := map[string]struct{ data, token string }{
		"пустые данные":     {"", testToken},
		"нет токена":        {valid, ""},
		"чужой токен":       {valid, "999:other"},
		"подделан user":     {strings.Replace(valid, "42", "43", 1), testToken},
		"без hash":          {"auth_date=1&user=%7B%22id%22%3A1%7D", testToken},
		"без user":          {noUser, testToken},
		"битый JSON в user": {badJSON, testToken},
	}
	for name, c := range cases {
		if _, ok := VerifyInitData(c.data, c.token); ok {
			t.Errorf("%s: должно быть отвергнуто", name)
		}
	}
}

func TestSendMessageWithoutTokenIsNoop(t *testing.T) {
	SendMessage("", "1", "hi") // не должно паниковать и ходить в сеть
	SendMessage("", "1", "hi")
	if !warnedOnce {
		t.Error("предупреждение об отсутствии BOT_TOKEN не залогировано")
	}
}
