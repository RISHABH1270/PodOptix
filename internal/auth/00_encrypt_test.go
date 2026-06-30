package auth

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

const testKey = "test-32-byte-encryption-key!!!!1" // exactly 32 bytes

func TestEncryptDecrypt_RoundTrip(t *testing.T) {
	plain := "eyJhbGciOiJSUzI1NiIsInR5cCI6IkpXVCJ9"

	encrypted, err := Encrypt(plain, testKey)
	assert.NoError(t, err)
	assert.NotEqual(t, plain, encrypted) // must not store plain text

	decrypted, err := Decrypt(encrypted, testKey)
	assert.NoError(t, err)
	assert.Equal(t, plain, decrypted)
}

func TestEncrypt_DifferentEachTime(t *testing.T) {
	plain := "same-token"

	enc1, _ := Encrypt(plain, testKey)
	enc2, _ := Encrypt(plain, testKey)

	// same input → different ciphertext each time (random nonce)
	assert.NotEqual(t, enc1, enc2)

	// but both decrypt to same value
	dec1, _ := Decrypt(enc1, testKey)
	dec2, _ := Decrypt(enc2, testKey)
	assert.Equal(t, plain, dec1)
	assert.Equal(t, plain, dec2)
}

func TestEncrypt_WrongKeyLength(t *testing.T) {
	_, err := Encrypt("token", "short-key")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "32 bytes")
}

func TestDecrypt_WrongKey(t *testing.T) {
	encrypted, _ := Encrypt("secret-token", testKey)

	wrongKey := "wrong-32-byte-encryption-key!!!1"
	_, err := Decrypt(encrypted, wrongKey)
	assert.Error(t, err)
}

func TestDecrypt_TamperedCiphertext(t *testing.T) {
	_, err := Decrypt("thisisnotvalidbase64orciphertext", testKey)
	assert.Error(t, err)
}
