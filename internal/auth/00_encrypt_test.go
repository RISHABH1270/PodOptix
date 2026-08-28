package auth

import (
	"fmt"
	"os"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
)

// ── test counter ──────────────────────────────────────────────────────────────

var (
	passed, failed, counter int
	mu                      sync.Mutex
	tty                     *os.File
)

func init() {
	var err error
	tty, err = os.OpenFile("/dev/tty", os.O_WRONLY, 0)
	if err != nil {
		tty = os.Stderr
	}
}

func log(format string, args ...any) { fmt.Fprintf(tty, format, args...) }

func shortName(full string) string {
	for i := len(full) - 1; i >= 0; i-- {
		if full[i] == '/' {
			return full[i+1:]
		}
	}
	return full
}

func track(t *testing.T) {
	t.Helper()
	mu.Lock()
	counter++
	n := counter
	mu.Unlock()
	t.Cleanup(func() {
		mu.Lock()
		defer mu.Unlock()
		if t.Failed() {
			failed++
			log("  [%2d]  ✗  %s\n", n, shortName(t.Name()))
		} else {
			passed++
			log("  [%2d]  ✓  %s\n", n, shortName(t.Name()))
		}
	})
}

func TestMain(m *testing.M) {
	log("\n  Running Auth Tests...\n")
	log("  ──────────────────────────────────────\n")
	code := m.Run()
	log("  Total: %d  |  Passed: %d  |  Failed: %d\n", passed+failed, passed, failed)
	if code == 0 {
		log("  ✓ All tests passed\n")
	} else {
		log("  ✗ Some tests failed\n")
	}
	log("  ──────────────────────────────────────\n\n")
	os.Exit(code)
}

// ── AES-256-GCM encrypt/decrypt ───────────────────────────────────────────────

const testKey = "test-32-byte-encryption-key!!!!1" // exactly 32 bytes

func TestEncryptDecrypt(t *testing.T) {
	t.Run("round trip — decrypt returns original plaintext", func(t *testing.T) {
		track(t)
		plain := "eyJhbGciOiJSUzI1NiIsInR5cCI6IkpXVCJ9"
		encrypted, err := Encrypt(plain, testKey)
		assert.NoError(t, err)
		assert.NotEqual(t, plain, encrypted) // must not store plain text

		decrypted, err := Decrypt(encrypted, testKey)
		assert.NoError(t, err)
		assert.Equal(t, plain, decrypted)
	})

	t.Run("same input produces different ciphertext each time — random nonce", func(t *testing.T) {
		track(t)
		enc1, _ := Encrypt("same-token", testKey)
		enc2, _ := Encrypt("same-token", testKey)
		assert.NotEqual(t, enc1, enc2)

		// but both decrypt to the same value
		dec1, _ := Decrypt(enc1, testKey)
		dec2, _ := Decrypt(enc2, testKey)
		assert.Equal(t, "same-token", dec1)
		assert.Equal(t, "same-token", dec2)
	})

	t.Run("wrong key length returns error", func(t *testing.T) {
		track(t)
		_, err := Encrypt("token", "short-key")
		assert.ErrorContains(t, err, "32 bytes")
	})

	t.Run("wrong decryption key returns error", func(t *testing.T) {
		track(t)
		encrypted, _ := Encrypt("secret-token", testKey)
		_, err := Decrypt(encrypted, "wrong-32-byte-encryption-key!!!1")
		assert.Error(t, err)
	})

	t.Run("tampered ciphertext returns error", func(t *testing.T) {
		track(t)
		_, err := Decrypt("thisisnotvalidbase64orciphertext", testKey)
		assert.Error(t, err)
	})
}
