package domain

import (
	"context"
	"encoding/json"
	"errors"
	"time"
)

// System configuration (#19, ADR-0005): settings of the outside world (LDAP now; the mail
// service and the object storage later) are kept in the database and edited by
// administrators, not in the configuration file.
//
// Storage: one row per section in system_config (the non-secret fields as JSON) and one row
// per secret field in system_config_secrets (AES-256-GCM, bound to section and field, with
// the version of the master key). Secrets are write-only: no API and no function in this file
// hands a stored secret to the HTTP layer; the domain decrypts them only to build the
// adapter that needs them (for LDAP: the directory).
//
// To add a section (SMTP, storage, ...): define its settings struct (JSON tags = stored
// fields), a validation function, and Get / Update / Test service methods built on
// loadSection / saveSection / loadSecrets / saveSecret below, record changes with
// configChangedAudit, and add the permission check requirePermission(actor, PermConfigManage).
// ldap_config.go is the model.

// Section names are stored in system_config.section and appear as the audit target id.
const (
	SectionLDAP    = "ldap"
	SectionStorage = "storage"
)

// StoredSecret is an encrypted secret as the repository keeps it.
type StoredSecret struct {
	Enc        []byte
	KeyVersion int
}

// SystemConfigRepository is the persistence port of the system configuration.
type SystemConfigRepository interface {
	// GetConfigSection returns the stored JSON of a section; ok=false when it was never saved.
	GetConfigSection(ctx context.Context, section string) (data []byte, updatedAt time.Time, ok bool, err error)
	// PutConfigSection stores (inserts or replaces) the JSON of a section.
	PutConfigSection(ctx context.Context, section string, data []byte, at time.Time) error
	// InsertConfigSectionIfAbsent stores the section only if it has no row yet (the
	// one-time import) and reports whether it did.
	InsertConfigSectionIfAbsent(ctx context.Context, section string, data []byte, at time.Time) (inserted bool, err error)
	// ListConfigSecrets returns the encrypted secrets of a section by field name.
	ListConfigSecrets(ctx context.Context, section string) (map[string]StoredSecret, error)
	PutConfigSecret(ctx context.Context, section, field string, secret StoredSecret, at time.Time) error
	DeleteConfigSecret(ctx context.Context, section, field string) error
}

// loadSection decodes the stored JSON of a section into dst and reports whether the section
// has ever been saved (dst is untouched when not).
func loadSection(ctx context.Context, repo Repository, section string, dst any) (saved bool, updatedAt time.Time, err error) {
	data, at, ok, err := repo.GetConfigSection(ctx, section)
	if err != nil || !ok {
		return false, time.Time{}, err
	}
	return true, at, json.Unmarshal(data, dst)
}

// configSecrets are the secrets of one section as the domain sees them: which fields are set,
// their plaintext (only for fields that could be decrypted), and which could not be read.
type configSecrets struct {
	set        map[string]bool
	plain      map[string]string
	unreadable map[string]bool
}

func (c configSecrets) anyUnreadable() bool { return len(c.unreadable) > 0 }

func (s *Service) loadSecrets(ctx context.Context, repo Repository, section string) (configSecrets, error) {
	stored, err := repo.ListConfigSecrets(ctx, section)
	if err != nil {
		return configSecrets{}, err
	}
	out := configSecrets{set: map[string]bool{}, plain: map[string]string{}, unreadable: map[string]bool{}}
	for field, sec := range stored {
		out.set[field] = true
		plain, err := s.tokens.openField(section, field, sec.Enc, sec.KeyVersion)
		switch {
		case err == nil:
			out.plain[field] = string(plain)
		case errors.Is(err, errSecretUnreadable):
			out.unreadable[field] = true
		default:
			return configSecrets{}, err
		}
	}
	return out, nil
}

// saveSecret encrypts and stores one secret field (replacing the old value).
func (s *Service) saveSecret(ctx context.Context, tx Repository, section, field, plain string) error {
	enc, version, err := s.tokens.sealField(section, field, []byte(plain))
	if err != nil {
		return err
	}
	return tx.PutConfigSecret(ctx, section, field, StoredSecret{Enc: enc, KeyVersion: version}, s.nowUTC())
}

// ConnectionTest is the answer of every "test connection" call. OK=false carries the error
// code (rendered by the frontend as "error.<code>") that says what failed, for example
// ldapUnreachable. A failed test is an expected outcome, not an error of the request.
type ConnectionTest struct {
	OK   bool
	Code string
}

func jsonBytes(v any) ([]byte, error) { return json.Marshal(v) }
