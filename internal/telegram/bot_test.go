package telegram

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func fakeAPI(t *testing.T, respond map[string]string) (*Bot, *[]string, *[]map[string]any) {
	t.Helper()
	var methods []string
	var bodies []map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.URL.Path, "/botTOKEN/") {
			t.Errorf("путь без токена: %s", r.URL.Path)
		}
		method := strings.TrimPrefix(r.URL.Path, "/botTOKEN/")
		methods = append(methods, method)
		b, _ := io.ReadAll(r.Body)
		var m map[string]any
		json.Unmarshal(b, &m)
		bodies = append(bodies, m)
		if res, ok := respond[method]; ok {
			w.Write([]byte(res))
			return
		}
		w.Write([]byte(`{"ok":true,"result":true}`))
	}))
	t.Cleanup(srv.Close)
	b := NewBot("TOKEN")
	b.BaseURL = srv.URL
	return b, &methods, &bodies
}

func TestBotCalls(t *testing.T) {
	b, methods, bodies := fakeAPI(t, map[string]string{
		"getMe":             `{"ok":true,"result":{"username":"version20bot"}}`,
		"createInvoiceLink": `{"ok":true,"result":"https://t.me/$abc"}`,
	})

	name, err := b.GetMe()
	if err != nil || name != "version20bot" {
		t.Fatalf("GetMe: %q %v", name, err)
	}
	link, err := b.CreateInvoiceLink(Invoice{Title: "Plus", Payload: "sub:1:plus369", Currency: "RUB",
		Prices: []LabeledPrice{{Label: "Plus", Amount: 36900}}, NeedEmail: true})
	if err != nil || link != "https://t.me/$abc" {
		t.Fatalf("CreateInvoiceLink: %q %v", link, err)
	}
	if err := b.AnswerPreCheckoutQuery("q1", false, "Сумма не совпадает"); err != nil {
		t.Fatal(err)
	}
	if err := b.SetWebhook("https://x/telegram/webhook", "sec"); err != nil {
		t.Fatal(err)
	}
	if err := b.SendMessageWithWebAppButton("42", "Привет", "Открыть", "https://x/?ref=A"); err != nil {
		t.Fatal(err)
	}

	want := []string{"getMe", "createInvoiceLink", "answerPreCheckoutQuery", "setWebhook", "sendMessage"}
	if strings.Join(*methods, ",") != strings.Join(want, ",") {
		t.Fatalf("методы: %v", *methods)
	}
	inv := (*bodies)[1]
	if inv["payload"] != "sub:1:plus369" || inv["need_email"] != true || inv["provider_token"] != nil {
		t.Errorf("инвойс: %v", inv)
	}
	if pc := (*bodies)[2]; pc["ok"] != false || pc["error_message"] != "Сумма не совпадает" {
		t.Errorf("pre_checkout: %v", pc)
	}
	if wh := (*bodies)[3]; wh["secret_token"] != "sec" {
		t.Errorf("webhook: %v", wh)
	}
	btn := (*bodies)[4]["reply_markup"].(map[string]any)["inline_keyboard"].([]any)[0].([]any)[0].(map[string]any)
	if btn["web_app"].(map[string]any)["url"] != "https://x/?ref=A" {
		t.Errorf("кнопка: %v", btn)
	}
}

func TestBotError(t *testing.T) {
	b, _, _ := fakeAPI(t, map[string]string{"getMe": `{"ok":false,"description":"Unauthorized"}`})
	if _, err := b.GetMe(); err == nil || !strings.Contains(err.Error(), "Unauthorized") {
		t.Fatalf("ошибка API не проброшена: %v", err)
	}
}
