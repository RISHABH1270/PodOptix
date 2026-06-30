package auth

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"io"
)

// Encrypt encrypts plain text using AES-256-GCM.
// The key must be exactly 32 bytes (256 bits).
// Returns base64-encoded ciphertext (safe to store in the database).
func Encrypt(plainText string, key string) (string, error) {
	if len(key) != 32 {
		return "", fmt.Errorf("encryption key must be exactly 32 bytes, got %d", len(key))
	}

	// create AES cipher block from the 32-byte key
	var block cipher.Block
	var err error
	block, err = aes.NewCipher([]byte(key))
	if err != nil {
		return "", fmt.Errorf("create cipher: %w", err)
	}

	// GCM mode — provides authenticated encryption (detects tampering)
	var gcm cipher.AEAD
	gcm, err = cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("create gcm: %w", err)
	}

	// generate a random nonce (number used once) — different every encryption
	var nonce []byte
	nonce = make([]byte, gcm.NonceSize())
	if _, err = io.ReadFull(rand.Reader, nonce); err != nil {
		return "", fmt.Errorf("generate nonce: %w", err)
	}

	// encrypt: nonce is prepended to ciphertext so we can extract it during decryption
	var cipherText []byte
	cipherText = gcm.Seal(nonce, nonce, []byte(plainText), nil)

	// base64 encode so it's safe to store as a string in PostgreSQL
	return base64.StdEncoding.EncodeToString(cipherText), nil
}

// Decrypt decrypts a base64-encoded AES-256-GCM ciphertext.
// The key must be exactly 32 bytes (256 bits).
func Decrypt(cipherTextB64 string, key string) (string, error) {
	if len(key) != 32 {
		return "", fmt.Errorf("encryption key must be exactly 32 bytes, got %d", len(key))
	}

	// decode from base64
	var cipherText []byte
	var err error
	cipherText, err = base64.StdEncoding.DecodeString(cipherTextB64)
	if err != nil {
		return "", fmt.Errorf("decode base64: %w", err)
	}

	// create AES cipher block
	var block cipher.Block
	block, err = aes.NewCipher([]byte(key))
	if err != nil {
		return "", fmt.Errorf("create cipher: %w", err)
	}

	// GCM mode
	var gcm cipher.AEAD
	gcm, err = cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("create gcm: %w", err)
	}

	// extract nonce from the beginning of ciphertext
	var nonceSize int
	nonceSize = gcm.NonceSize()
	if len(cipherText) < nonceSize {
		return "", fmt.Errorf("ciphertext too short")
	}

	var nonce []byte
	var encrypted []byte
	nonce, encrypted = cipherText[:nonceSize], cipherText[nonceSize:]

	// decrypt and verify authentication tag
	var plainText []byte
	plainText, err = gcm.Open(nil, nonce, encrypted, nil)
	if err != nil {
		return "", fmt.Errorf("decrypt: %w", err)
	}

	return string(plainText), nil
}
