package domain

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"strconv"
	"strings"
)

// Recovery codes (#31, ADR-0006). A user with a second factor has a set of one-time codes that
// stand in for the authenticator's code when the phone is lost. They are made when the second
// factor is turned on and again on request (the old set is void then); the plain codes are
// returned once and never stored: only their hashes are.

const (
	recoveryCodeCount = 10
	// A code is 12 symbols of a 32-symbol alphabet: 60 bits from the system's random source.
	// A fast hash (SHA-256, the same as for session tokens) is enough for something this long
	// and random: the hashes cannot be turned back into codes by guessing.
	recoveryCodeLength   = 12
	recoveryCodeAlphabet = "23456789ABCDEFGHJKLMNPQRSTUVWXYZ" // no 0/O, 1/I
)

// RecoveryCode is a stored, unspent recovery code.
type RecoveryCode struct {
	ID   int64
	Hash string
}

// newRecoveryCode makes one code, in the form shown to the user: XXXX-XXXX-XXXX.
func newRecoveryCode() (string, error) {
	raw := make([]byte, recoveryCodeLength)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	var b strings.Builder
	for i, v := range raw {
		if i > 0 && i%4 == 0 {
			b.WriteByte('-')
		}
		b.WriteByte(recoveryCodeAlphabet[v&31]) // 32 symbols: no modulo bias
	}
	return b.String(), nil
}

// normalizeRecoveryCode makes what the user typed comparable: no spaces or dashes, upper case.
func normalizeRecoveryCode(s string) string {
	s = strings.ToUpper(s)
	return strings.NewReplacer("-", "", " ", "").Replace(s)
}

// hashRecoveryCode is the stored form of a code. The user's ID is part of it, so equal codes
// of two users never share a hash.
func hashRecoveryCode(id UserID, code string) string {
	sum := sha256.Sum256([]byte("recovery:" + strconv.FormatInt(int64(id), 10) + ":" + normalizeRecoveryCode(code)))
	return hex.EncodeToString(sum[:])
}

// replaceRecoveryCodes makes a new set for the user, voiding the old one, and returns the
// plain codes. Call inside InTx.
func (s *Service) replaceRecoveryCodes(ctx context.Context, tx Repository, id UserID) ([]string, error) {
	codes := make([]string, recoveryCodeCount)
	hashes := make([]string, recoveryCodeCount)
	for i := range codes {
		c, err := newRecoveryCode()
		if err != nil {
			return nil, err
		}
		codes[i], hashes[i] = c, hashRecoveryCode(id, c)
	}
	if err := tx.ReplaceRecoveryCodes(ctx, id, hashes, s.nowUTC()); err != nil {
		return nil, err
	}
	return codes, nil
}
