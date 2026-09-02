package testkit

import (
	"testing"

	"github.com/RISHABH1270/PodOptix/internal/auth"
	"github.com/stretchr/testify/assert"
)

const encTestKey = "test-32-byte-encryption-key!!!!1"

func TestEncryptDecrypt(t *testing.T) {
	t.Run("round trip — decrypt returns original plaintext", func(t *testing.T) {
		track(t)
		plain := "eyJhbGciOiJSUzI1NiIsInR5cCI6IkpXVCJ9"
		encrypted, err := auth.Encrypt(plain, encTestKey)
		assert.NoError(t, err)
		assert.NotEqual(t, plain, encrypted)

		decrypted, err := auth.Decrypt(encrypted, encTestKey)
		assert.NoError(t, err)
		assert.Equal(t, plain, decrypted)
	})

	t.Run("same input produces different ciphertext each time — random nonce", func(t *testing.T) {
		track(t)
		enc1, _ := auth.Encrypt("same-token", encTestKey)
		enc2, _ := auth.Encrypt("same-token", encTestKey)
		assert.NotEqual(t, enc1, enc2)
		dec1, _ := auth.Decrypt(enc1, encTestKey)
		dec2, _ := auth.Decrypt(enc2, encTestKey)
		assert.Equal(t, "same-token", dec1)
		assert.Equal(t, "same-token", dec2)
	})

	t.Run("wrong key length returns error", func(t *testing.T) {
		track(t)
		_, err := auth.Encrypt("token", "short-key")
		assert.ErrorContains(t, err, "32 bytes")
	})

	t.Run("wrong decryption key returns error", func(t *testing.T) {
		track(t)
		encrypted, _ := auth.Encrypt("secret-token", encTestKey)
		_, err := auth.Decrypt(encrypted, "wrong-32-byte-encryption-key!!!1")
		assert.Error(t, err)
	})

	t.Run("tampered ciphertext returns error", func(t *testing.T) {
		track(t)
		_, err := auth.Decrypt("thisisnotvalidbase64orciphertext", encTestKey)
		assert.Error(t, err)
	})
}
