package domain

import "context"

// SeedTOTP gives the user an enabled authenticator without any of the effects of the setup
// flow (no audit entry, no mail, no recovery codes) and returns its base32 secret. TEST ONLY
// FIXTURE, like EnqueueMail: the API tests use it so that the always-forced test administrator
// does not put an enrolment into every audit log they look at. The setup flow itself is
// tested through its API.
func (s *Service) SeedTOTP(ctx context.Context, userID UserID) (string, error) {
	secret, err := newTOTPSecret()
	if err != nil {
		return "", err
	}
	enc, version, err := s.tokens.sealField(tokenSecretSection, totpSecretField(userID), secret)
	if err != nil {
		return "", err
	}
	err = s.repo.InTx(ctx, func(tx Repository) error {
		if err := tx.ReplacePendingAuthenticator(ctx, Authenticator{
			UserID: userID, Type: AuthenticatorTOTP, SecretEnc: enc, KeyVersion: version, CreatedAt: s.nowUTC(),
		}); err != nil {
			return err
		}
		all, err := tx.ListAuthenticators(ctx, userID)
		if err != nil {
			return err
		}
		for _, a := range all {
			if a.Type == AuthenticatorTOTP && !a.Enabled {
				_, err := tx.EnableAuthenticator(ctx, a.ID, 0, s.nowUTC())
				return err
			}
		}
		return nil
	})
	return totpEncoding.EncodeToString(secret), err
}
