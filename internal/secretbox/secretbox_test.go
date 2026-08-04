package secretbox

import (
	"bytes"
	"testing"
)

func TestSealOpenAndContextBinding(t *testing.T) {
	encoded, err := GenerateKey()
	if err != nil {
		t.Fatal(err)
	}
	key, err := ParseKey(encoded)
	if err != nil {
		t.Fatal(err)
	}
	box, err := New(key)
	if err != nil {
		t.Fatal(err)
	}
	ciphertext, err := box.Seal([]byte("private material"), "credentials:test")
	if err != nil {
		t.Fatal(err)
	}
	plaintext, err := box.Open(ciphertext, "credentials:test")
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(plaintext, []byte("private material")) {
		t.Fatalf("unexpected plaintext: %q", plaintext)
	}
	if _, err := box.Open(ciphertext, "credentials:other"); err == nil {
		t.Fatal("expected associated-context authentication failure")
	}
	mutated := append([]byte(nil), ciphertext...)
	mutated[len(mutated)-1] ^= 1
	if _, err := box.Open(mutated, "credentials:test"); err == nil {
		t.Fatal("expected tamper detection")
	}
}
