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
//
// version is the master key version (XP_SECRET_KEY_VERSION). Secrets of the system
// configuration (system_config.go) record it next to the ciphertext and are bound to their
// section and field (sealField / openField).
type tokenCipher struct {
	aead    cipher.AEAD
	version int
}

func newTokenCipher(secret string) (tokenCipher, error) {
	if secret == "" {
		return tokenCipher{}, errors.New("secret key is required (XP_SECRET_KEY or XP_SECRET_KEY_FILE)")
	}
	key := sha256.Sum256([]byte(secret))
	block, err := aes.NewCipher(key[:])
	if err != nil {
		return tokenCipher{}, err
	}
	aead, err := cipher.NewGCM(block)
	return tokenCipher{aead: aead, version: 1}, err
}

// sealField encrypts the secret value of one configuration field. The section and field
// are authenticated with it, so a stored value cannot be moved to another field. It
// returns the ciphertext and the key version to store next to it.
func (c tokenCipher) sealField(section, field string, plain []byte) (sealed []byte, keyVersion int, err error) {
	if c.aead == nil {
		return nil, 0, errors.New("secret key is not configured")
	}
	nonce := make([]byte, c.aead.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, 0, err
	}
	return c.aead.Seal(nonce, nonce, plain, []byte(section+"/"+field)), c.version, nil
}

// errSecretUnreadable: the stored secret was sealed with another master key (or key
// version), or was damaged. The administrator has to enter it again.
var errSecretUnreadable = errors.New("stored secret cannot be decrypted with the current master key")

func (c tokenCipher) openField(section, field string, sealed []byte, keyVersion int) ([]byte, error) {
	if c.aead == nil {
		return nil, errors.New("secret key is not configured")
	}
	n := c.aead.NonceSize()
	if keyVersion != c.version || len(sealed) < n {
		return nil, errSecretUnreadable
	}
	plain, err := c.aead.Open(nil, sealed[:n], sealed[n:], []byte(section+"/"+field))
	if err != nil {
		return nil, errSecretUnreadable
	}
	return plain, nil
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
