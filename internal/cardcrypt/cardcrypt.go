// Package cardcrypt шифрует номера карт для выплат (AES-256-GCM). Ключ —
// 32 байта из CARD_ENC_KEY в base64 (openssl rand -base64 32). В БД лежит
// только base64(nonce || шифртекст); без ключа расшифровать нельзя.
package cardcrypt

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"strings"
)

type Key struct {
	aead cipher.AEAD
}

// ParseKey разбирает CARD_ENC_KEY. Пустая строка — (nil, nil): вывод выключен.
func ParseKey(b64 string) (*Key, error) {
	b64 = strings.TrimSpace(b64)
	if b64 == "" {
		return nil, nil
	}
	raw, err := base64.StdEncoding.DecodeString(b64)
	if err != nil {
		return nil, errors.New("CARD_ENC_KEY: не base64")
	}
	if len(raw) != 32 {
		return nil, errors.New("CARD_ENC_KEY: нужно ровно 32 байта (openssl rand -base64 32)")
	}
	block, err := aes.NewCipher(raw)
	if err != nil {
		return nil, err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	return &Key{aead: aead}, nil
}

func (k *Key) Encrypt(plain string) (string, error) {
	nonce := make([]byte, k.aead.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return "", err
	}
	sealed := k.aead.Seal(nonce, nonce, []byte(plain), nil)
	return base64.StdEncoding.EncodeToString(sealed), nil
}

func (k *Key) Decrypt(enc string) (string, error) {
	raw, err := base64.StdEncoding.DecodeString(enc)
	if err != nil {
		return "", err
	}
	n := k.aead.NonceSize()
	if len(raw) < n {
		return "", errors.New("шифртекст слишком короткий")
	}
	plain, err := k.aead.Open(nil, raw[:n], raw[n:], nil)
	if err != nil {
		return "", err
	}
	return string(plain), nil
}
