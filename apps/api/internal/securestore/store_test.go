package securestore

import (
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
	tampered := ciphertext[:len(ciphertext)-1] + "A"
	if _, err := store.Decrypt(tampered, 1, "field"); err == nil {
		t.Fatal("tampering must fail")
	}
	if _, err := store.Decrypt("v2:AAAA", 1, "field"); err == nil {
		t.Fatal("unknown version must fail")
	}
}
