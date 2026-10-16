package domain

import "context"

// Resetting a second factor (#23, ADR-0006): the way back in for a user who lost the
// authenticator and the recovery codes. An administrator does it from user management; the
// command line does it for an administrator who is locked out (cli.go).

// resetSecondFactor removes the user's second factor and everything attached to it (pending
// authenticators, recovery codes, the failed-attempt counter), records who did it and tells
// the contact email when mail works. Call inside InTx. The user's open sessions stay: they were
// opened with a second factor; the user is not logged out by the loss of a device.
func (s *Service) resetSecondFactor(ctx context.Context, tx Repository, by AuditActor, u User) error {
	if err := tx.DeleteAuthenticatorsAndRecoveryCodes(ctx, u.ID); err != nil {
		return err
	}
	if err := tx.PutSecondFactorLockout(ctx, u.ID, SecondFactorLockout{}); err != nil {
		return err
	}
	if err := s.recordAudit(ctx, tx, by, secondFactorResetAudit(u.ID)); err != nil {
		return err
	}
	return s.notifySecurity(ctx, tx, u.ID, u.Email, MailTemplateSecondFactorReset, nil)
}

// ResetSecondFactor removes a user's second factor. Administrators only (user.manage). The user
// is told by mail when mail works and the reset is audited with the administrator as operator.
// A user who is required to have a second factor sets it up again at their next request.
//
// Errors: 403 forbidden, 404 for an unknown user or one without a second factor.
func (s *Service) ResetSecondFactor(ctx context.Context, actor Actor, id UserID) (UserWithIdentities, error) {
	if err := requirePermission(actor, PermUserManage); err != nil {
		return UserWithIdentities{}, err
	}
	var result UserWithIdentities
	err := s.repo.InTx(ctx, func(tx Repository) error {
		cur, found, err := tx.LockUserWithIdentities(ctx, id)
		if err != nil {
			return err
		}
		if !found {
			return newKind(KindNotFound, CodeNotFound)
		}
		if err := requireEnabledSecondFactor(ctx, tx, id); err != nil {
			return err
		}
		if err := s.resetSecondFactor(ctx, tx, AuditActorOf(actor), cur.User); err != nil {
			return err
		}
		cur.SecondFactorEnabled = false
		result = cur
		return nil
	})
	return result, err
}
