package cardcrypt

import (
	"encoding/base64"
	"strings"
	"testing"
)

var testKey = base64.StdEncoding.EncodeToString([]byte("0123456789abcdef0123456789abcdef"))

func TestRoundTrip(t *testing.T) {
	k, err := ParseKey(testKey)
	if err != nil || k == nil {
		t.Fatal(err)
	}
	a, _ := k.Encrypt("2200123412341234")
	b, _ := k.Encrypt("2200123412341234")
	if a == b {
		t.Error("одинаковый шифртекст для одной карты — нет случайного nonce")
	}
	if strings.Contains(a, "2200") {
		t.Error("номер карты виден в шифртексте")
	}
	for _, enc := range []string{a, b} {
		if got, err := k.Decrypt(enc); err != nil || got != "2200123412341234" {
			t.Errorf("Decrypt: %q %v", got, err)
		}
	}

	other, _ := ParseKey(base64.StdEncoding.EncodeToString([]byte("fedcba9876543210fedcba9876543210")))
	if _, err := other.Decrypt(a); err == nil {
		t.Error("чужой ключ расшифровал")
	}
	if _, err := k.Decrypt("бред"); err == nil {
		t.Error("мусор расшифрован")
	}
}

func TestParseKey(t *testing.T) {
	if k, err := ParseKey(""); k != nil || err != nil {
		t.Error("пустой ключ должен выключать вывод без ошибки")
	}
	for _, bad := range []string{"не-base64!", base64.StdEncoding.EncodeToString([]byte("short"))} {
		if _, err := ParseKey(bad); err == nil {
			t.Errorf("ключ %q принят", bad)
		}
	}
}
