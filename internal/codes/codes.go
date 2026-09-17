// Package codes генерирует короткие случайные коды приглашений.
// Порт backend/lib/codes.js.
package codes

import (
	"crypto/rand"
	"encoding/base64"
	"strings"
)

func Generate(length int) string {
	buf := make([]byte, length)
	_, _ = rand.Read(buf)
	s := base64.RawURLEncoding.EncodeToString(buf)
	if len(s) > length {
		s = s[:length]
	}
	return strings.ToUpper(s)
}
