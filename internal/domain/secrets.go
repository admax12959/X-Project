package domain

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"errors"
)

// tokenCipher encrypts secrets stored in the database (code platform tokens) with
// AES-256-GCM. The key is derived from the configured secret key (XP_SECRET_KEY).
// Sealed form: nonce || ciphertext+tag, with a fresh random nonce per call.
// Changing the secret key makes existing sealed values unreadable.
type tokenCipher struct{ aead cipher.AEAD }

func newTokenCipher(secret string) (tokenCipher, error) {
	if secret == "" {
		return tokenCipher{}, errors.New("secret key is required (secret_key or XP_SECRET_KEY)")
	}
	key := sha256.Sum256([]byte(secret))
	block, err := aes.NewCipher(key[:])
	if err != nil {
		return tokenCipher{}, err
	}
	aead, err := cipher.NewGCM(block)
	return tokenCipher{aead: aead}, err
}

func (c tokenCipher) seal(plain []byte) ([]byte, error) {
	if c.aead == nil {
		return nil, errors.New("secret key is not configured")
	}
	nonce := make([]byte, c.aead.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, err
	}
	return c.aead.Seal(nonce, nonce, plain, nil), nil
}

func (c tokenCipher) open(sealed []byte) ([]byte, error) {
	if c.aead == nil {
		return nil, errors.New("secret key is not configured")
	}
	n := c.aead.NonceSize()
	if len(sealed) < n {
		return nil, errors.New("sealed value too short")
	}
	return c.aead.Open(nil, sealed[:n], sealed[n:], nil)
}
