package domain

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
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

// isTOTPCode says whether what the user typed has the shape of an authenticator code: six
// digits, spaces ignored. Anything else is looked up among the recovery codes.
func isTOTPCode(code string) bool {
	code = strings.ReplaceAll(strings.TrimSpace(code), " ", "")
	if len(code) != totpDigits {
		return false
	}
	for _, r := range code {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

// claimRecoveryCode spends the unspent recovery code of the user that equals code. Every
// stored hash is compared in constant time, with no early exit, so the time taken does not
// tell which one matched. Of two requests with the same code only one gets it.
func (s *Service) claimRecoveryCode(ctx context.Context, id UserID, code string) (bool, error) {
	stored, err := s.repo.ListUnusedRecoveryCodes(ctx, id)
	if err != nil {
		return false, err
	}
	want := []byte(hashRecoveryCode(id, code))
	var hit *RecoveryCode
	for i := range stored {
		if subtle.ConstantTimeCompare(want, []byte(stored[i].Hash)) == 1 {
			hit = &stored[i]
		}
	}
	if hit == nil {
		return false, nil
	}
	return s.repo.ClaimRecoveryCode(ctx, hit.ID, s.nowUTC())
}
