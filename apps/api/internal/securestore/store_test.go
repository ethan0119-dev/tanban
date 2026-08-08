package securestore

import (
	"encoding/base64"
	"strings"
	"testing"
)

func TestEncryptDecryptBindsTenantAndPurpose(t *testing.T) {
	store, err := New(strings.Repeat("k", 32))
	if err != nil {
		t.Fatal(err)
	}
	ciphertext, err := store.Encrypt([]byte("6222020202020202"), 7, "wechat-onboarding")
	if err != nil {
		t.Fatal(err)
	}
	plaintext, err := store.Decrypt(ciphertext, 7, "wechat-onboarding")
	if err != nil || string(plaintext) != "6222020202020202" {
		t.Fatalf("decrypt=%q err=%v", plaintext, err)
	}
	if _, err = store.Decrypt(ciphertext, 8, "wechat-onboarding"); err == nil {
		t.Fatal("tenant swap must fail")
	}
	if _, err = store.Decrypt(ciphertext, 7, "another-purpose"); err == nil {
		t.Fatal("purpose swap must fail")
	}
}

func TestDecryptRejectsTamperingAndUnknownVersion(t *testing.T) {
	store, _ := New(strings.Repeat("z", 32))
	ciphertext, _ := store.Encrypt([]byte("secret"), 1, "field")
	_, encoded, _ := strings.Cut(ciphertext, ":")
	payload, _ := base64.RawURLEncoding.DecodeString(encoded)
	payload[len(payload)-1] ^= 0x01
	tampered := "v1:" + base64.RawURLEncoding.EncodeToString(payload)
	if _, err := store.Decrypt(tampered, 1, "field"); err == nil {
		t.Fatal("tampering must fail")
	}
	if _, err := store.Decrypt("v2:AAAA", 1, "field"); err == nil {
		t.Fatal("unknown version must fail")
	}
}
