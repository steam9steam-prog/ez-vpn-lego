package secretbox

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
)

const formatVersion byte = 1

type Box struct {
	aead cipher.AEAD
}

func ParseKey(encoded string) ([]byte, error) {
	key, err := base64.RawURLEncoding.DecodeString(encoded)
	if err != nil {
		return nil, fmt.Errorf("decode master key: %w", err)
	}
	if len(key) != 32 {
		return nil, errors.New("master key must contain exactly 32 bytes")
	}
	return key, nil
}

func GenerateKey() (string, error) {
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		return "", fmt.Errorf("generate master key: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(key), nil
}

func New(key []byte) (*Box, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("create secret cipher: %w", err)
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("create secret AEAD: %w", err)
	}
	return &Box{aead: aead}, nil
}

func (box *Box) Seal(plaintext []byte, context string) ([]byte, error) {
	nonce := make([]byte, box.aead.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, fmt.Errorf("generate secret nonce: %w", err)
	}
	result := make([]byte, 1, 1+len(nonce)+len(plaintext)+box.aead.Overhead())
	result[0] = formatVersion
	result = append(result, nonce...)
	return box.aead.Seal(result, nonce, plaintext, []byte(context)), nil
}

func (box *Box) Open(ciphertext []byte, context string) ([]byte, error) {
	minimum := 1 + box.aead.NonceSize() + box.aead.Overhead()
	if len(ciphertext) < minimum || ciphertext[0] != formatVersion {
		return nil, errors.New("unsupported or truncated encrypted secret")
	}
	nonce := ciphertext[1 : 1+box.aead.NonceSize()]
	plaintext, err := box.aead.Open(nil, nonce, ciphertext[1+box.aead.NonceSize():], []byte(context))
	if err != nil {
		return nil, errors.New("decrypt secret: authentication failed")
	}
	return plaintext, nil
}
