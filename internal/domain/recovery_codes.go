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

// ---------------------------------------------------------------- regenerate and disable (#31)

// secondFactorEnforcedFor says whether the user is required to have a second factor, in which
// case they cannot turn it off.
//
// TODO(#23): the enforcement rule (by role, department or user) lands with #23. Until then no
// one is required, and this is the one place that ticket has to fill in.
func (s *Service) secondFactorEnforcedFor(ctx context.Context, u User) (bool, error) {
	return false, nil
}

// requireEnabledSecondFactor says notFound for a user without an enabled authenticator, before
// any code is looked at: a wrong code of such a user must not count against their lockout.
func requireEnabledSecondFactor(ctx context.Context, tx Repository, id UserID) error {
	if on, err := hasEnabledTOTP(ctx, tx, id); err != nil {
		return err
	} else if !on {
		return newKind(KindNotFound, CodeNotFound)
	}
	return nil
}

// proveSecondFactor is the re-verification of a user who has a second factor: an authenticator
// code or an unspent recovery code, through the same lockout as the login's second step.
func (s *Service) proveSecondFactor(ctx context.Context, u User, code string) error {
	if err := requireEnabledSecondFactor(ctx, s.repo, u.ID); err != nil {
		return err
	}
	return s.verifySecondFactorCode(ctx, u, code)
}

// RegenerateRecoveryCodes replaces the actor's recovery codes after they proved themselves with
// a code (authenticator or recovery). The old set is void at once; the new plain codes are
// returned this once. The contact email is told when mail works.
//
// Errors: 404 without a second factor, 422 code required / totpInvalid, 429 totpLocked.
func (s *Service) RegenerateRecoveryCodes(ctx context.Context, actor Actor, code string) ([]string, error) {
	u := actor.User
	if err := s.proveSecondFactor(ctx, u, code); err != nil {
		return nil, err
	}
	var codes []string
	err := s.repo.InTx(ctx, func(tx Repository) error {
		if err := requireEnabledSecondFactor(ctx, tx, u.ID); err != nil {
			return err
		}
		var err error
		if codes, err = s.replaceRecoveryCodes(ctx, tx, u.ID); err != nil {
			return err
		}
		if err := tx.PutSecondFactorLockout(ctx, u.ID, SecondFactorLockout{}); err != nil {
			return err
		}
		if err := s.recordAudit(ctx, tx, AuditActorOf(actor), recoveryCodesRegeneratedAudit(u.ID)); err != nil {
			return err
		}
		return s.notifySecurity(ctx, tx, u.ID, u.Email, MailTemplateRecoveryCodesRegenerated, nil)
	})
	if err != nil {
		return nil, err
	}
	return codes, nil
}

// DisableSecondFactor turns the actor's second factor off, removing the authenticator and the
// recovery codes, after they proved themselves with a code. A user the rules require to have a
// second factor is refused (403 secondFactorEnforced) before any code is looked at.
//
// Errors: 404 without a second factor, 403 secondFactorEnforced, 422 code required /
// totpInvalid, 429 totpLocked.
func (s *Service) DisableSecondFactor(ctx context.Context, actor Actor, code string) error {
	u := actor.User
	if err := requireEnabledSecondFactor(ctx, s.repo, u.ID); err != nil {
		return err
	}
	if enforced, err := s.secondFactorEnforcedFor(ctx, u); err != nil {
		return err
	} else if enforced {
		return newKind(KindForbidden, CodeSecondFactorEnforced)
	}
	if err := s.proveSecondFactor(ctx, u, code); err != nil {
		return err
	}
	return s.repo.InTx(ctx, func(tx Repository) error {
		if err := requireEnabledSecondFactor(ctx, tx, u.ID); err != nil {
			return err
		}
		if err := tx.DeleteAuthenticatorsAndRecoveryCodes(ctx, u.ID); err != nil {
			return err
		}
		if err := tx.PutSecondFactorLockout(ctx, u.ID, SecondFactorLockout{}); err != nil {
			return err
		}
		if err := s.recordAudit(ctx, tx, AuditActorOf(actor), secondFactorDisabledAudit(u.ID)); err != nil {
			return err
		}
		return s.notifySecurity(ctx, tx, u.ID, u.Email, MailTemplateSecondFactorDisabled, nil)
	})
}
