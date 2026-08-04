// Package crypto encrypts the bucket credentials users hand us for their own
// R2/S3 storage. Those keys are the most dangerous thing in this database:
// they grant write access to someone else's paid bucket, so they are never
// stored in plaintext and never returned to a client.
//
// Scheme is AES-256-GCM with a per-record random nonce, prefixed to the
// ciphertext. The master key comes from STORAGE_MASTER_KEY (base64, 32 bytes)
// and lives only in the environment file — losing it makes every stored
// credential unrecoverable, which is the intended failure mode.
package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"strings"
)

var (
	ErrNoKey       = errors.New("crypto: master key not configured (set STORAGE_MASTER_KEY)")
	ErrCiphertext  = errors.New("crypto: ciphertext malformed")
	ErrKeyNotBytes = errors.New("crypto: master key must decode to exactly 32 bytes")
)

// Cipher holds the AEAD built from the master key. A nil *Cipher is valid and
// reports ErrNoKey from every operation, so a server without BYOS configured
// still boots — it just can't store user credentials.
type Cipher struct {
	aead cipher.AEAD
}

// New builds a Cipher from a base64-encoded 32-byte key. An empty key yields
// (nil, nil): BYOS stays disabled but the rest of the server runs.
func New(base64Key string) (*Cipher, error) {
	base64Key = strings.TrimSpace(base64Key)
	if base64Key == "" {
		return nil, nil
	}
	key, err := base64.StdEncoding.DecodeString(base64Key)
	if err != nil {
		// Tolerate the URL-safe alphabet — easy to paste the wrong one.
		key, err = base64.URLEncoding.DecodeString(base64Key)
		if err != nil {
			return nil, fmt.Errorf("crypto: decode master key: %w", err)
		}
	}
	if len(key) != 32 {
		return nil, ErrKeyNotBytes
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	return &Cipher{aead: aead}, nil
}

// Enabled reports whether credentials can be stored.
func (c *Cipher) Enabled() bool { return c != nil && c.aead != nil }

// Encrypt returns nonce||ciphertext. Empty input encrypts to nil so an unset
// credential stays unset rather than becoming an encrypted empty string.
func (c *Cipher) Encrypt(plaintext string) ([]byte, error) {
	if !c.Enabled() {
		return nil, ErrNoKey
	}
	if plaintext == "" {
		return nil, nil
	}
	nonce := make([]byte, c.aead.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, err
	}
	return c.aead.Seal(nonce, nonce, []byte(plaintext), nil), nil
}

// Decrypt reverses Encrypt. Nil/empty input decrypts to "".
func (c *Cipher) Decrypt(blob []byte) (string, error) {
	if len(blob) == 0 {
		return "", nil
	}
	if !c.Enabled() {
		return "", ErrNoKey
	}
	ns := c.aead.NonceSize()
	if len(blob) < ns+1 {
		return "", ErrCiphertext
	}
	out, err := c.aead.Open(nil, blob[:ns], blob[ns:], nil)
	if err != nil {
		return "", fmt.Errorf("%w: %v", ErrCiphertext, err)
	}
	return string(out), nil
}

// Mask renders a credential for display: first 4 and last 4 characters with
// the middle elided. Short secrets are fully masked rather than half-leaked.
func Mask(s string) string {
	if len(s) <= 8 {
		return strings.Repeat("*", len(s))
	}
	return s[:4] + strings.Repeat("*", 6) + s[len(s)-4:]
}

// GenerateKey produces a fresh base64 master key, for the ops docs and the
// `-genkey` startup flag.
func GenerateKey() (string, error) {
	key := make([]byte, 32)
	if _, err := io.ReadFull(rand.Reader, key); err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(key), nil
}
