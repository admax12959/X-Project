package domain

import (
	"context"
	"time"
)

// The security policy (#23, ADR-0006): whether a second factor is required. Administrators
// always need one, whatever is saved here; the one setting is "everybody needs one".
// It is a section of the system configuration (system_config.go, ADR-0005) without secrets.
//
// A user who is required to have a second factor and has none is let in with their password
// but can only set it up: the HTTP layer refuses everything else with 403
// secondFactorSetupRequired (Actor.SecondFactorSetupRequired). A user who has one cannot turn
// it off (403 secondFactorEnforced); an administrator can still reset it (secondfactor_reset.go).

// SecurityPolicy is the stored form of the policy. The JSON tags are the stored format and the
// API names.
type SecurityPolicy struct {
	ForceSecondFactor bool `json:"forceSecondFactor"`
}

// SecurityPolicyView is what the administrator's security page shows.
type SecurityPolicyView struct {
	SecurityPolicy
	Configured bool // saved at least once
	UpdatedAt  time.Time
}

func (s *Service) readSecurityPolicy(ctx context.Context, repo Repository) (p SecurityPolicy, saved bool, at time.Time, err error) {
	saved, at, err = loadSection(ctx, repo, SectionSecurity, &p)
	return p, saved, at, err
}

// GetSecurityPolicy returns the policy. Administrators only.
func (s *Service) GetSecurityPolicy(ctx context.Context, actor Actor) (SecurityPolicyView, error) {
	if err := requirePermission(actor, PermConfigManage); err != nil {
		return SecurityPolicyView{}, err
	}
	p, saved, at, err := s.readSecurityPolicy(ctx, s.repo)
	return SecurityPolicyView{SecurityPolicy: p, Configured: saved, UpdatedAt: at}, err
}

// UpdateSecurityPolicy saves the policy and, when it changed, records which field in the audit
// log. It is in effect at once, for sessions that are already open too. Administrators only.
func (s *Service) UpdateSecurityPolicy(ctx context.Context, actor Actor, in SecurityPolicy) (SecurityPolicyView, error) {
	if err := requirePermission(actor, PermConfigManage); err != nil {
		return SecurityPolicyView{}, err
	}
	var view SecurityPolicyView
	err := s.repo.InTx(ctx, func(tx Repository) error {
		old, _, _, err := s.readSecurityPolicy(ctx, tx)
		if err != nil {
			return err
		}
		data, err := jsonBytes(in)
		if err != nil {
			return err
		}
		if err := tx.PutConfigSection(ctx, SectionSecurity, data, s.nowUTC()); err != nil {
			return err
		}
		if old.ForceSecondFactor != in.ForceSecondFactor {
			if err := s.recordAudit(ctx, tx, AuditActorOf(actor), configChangedAudit(SectionSecurity, []string{"forceSecondFactor"})); err != nil {
				return err
			}
		}
		p, saved, at, err := s.readSecurityPolicy(ctx, tx)
		view = SecurityPolicyView{SecurityPolicy: p, Configured: saved, UpdatedAt: at}
		return err
	})
	return view, err
}

// secondFactorEnforcedFor says whether the user is required to have a second factor, in which
// case they cannot turn it off: administrators always, everybody when the policy says so.
func (s *Service) secondFactorEnforcedFor(ctx context.Context, u User) (bool, error) {
	if u.IsAdmin {
		return true, nil
	}
	p, _, _, err := s.readSecurityPolicy(ctx, s.repo)
	return p.ForceSecondFactor, err
}

// secondFactorSetupRequiredFor says whether the user is required to have a second factor and
// has none yet.
func (s *Service) secondFactorSetupRequiredFor(ctx context.Context, u User) (bool, error) {
	enforced, err := s.secondFactorEnforcedFor(ctx, u)
	if err != nil || !enforced {
		return false, err
	}
	on, err := hasEnabledTOTP(ctx, s.repo, u.ID)
	return !on, err
}
