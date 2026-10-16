package domain

import (
	"context"
	"reflect"
	"strings"
	"time"
)

// The skeleton shared by the system configuration sections that have secrets (LDAP, SMTP,
// storage; system_config.go has the storage model and the encryption). A section declares
//
//   - its settings struct (JSON tags = stored format = API names; the audit diff is generated
//     from the fields, so a new field cannot be forgotten there),
//   - its secrets (name, and when a stored one is dropped),
//   - a validator.
//
// and the skeleton does the rest of a save: read the old settings and secrets, validate, diff,
// store, keep or replace or delete the secrets, audit, read the result back, all in one
// transaction.

// sectionSecret declares one secret field of a section.
type sectionSecret[S any] struct {
	name string
	// unusedWhen reports that the settings no longer use the secret (an LDAP section without a
	// bind DN has no password to keep); a stored secret is then deleted. nil: never deleted.
	unusedWhen func(S) bool
}

// configSection declares one section. S is its settings, In the request of a save or a test.
type configSection[S, In any] struct {
	name     string
	defaults func() S // settings of a section that was never saved; nil: the zero value
	fill     func(*S) // fills empty fields with defaults after reading; may be nil
	secrets  []sectionSecret[S]
	// typed returns the secrets the request carries by field name; "" keeps the stored one.
	typed func(In) map[string]string
	// validate checks the request against the stored secrets and returns the settings to save
	// and the secret to use for each field (the typed one, else the stored one, "" for none).
	validate func(in In, stored configSecrets) (S, map[string]string, error)
}

// sectionState is a section as stored: settings and secrets read together.
type sectionState[S any] struct {
	settings S
	saved    bool // saved at least once; false = defaults
	at       time.Time
	secrets  configSecrets
}

// readSettings returns the stored settings of a section (defaults when never saved).
func readSettings[S, In any](ctx context.Context, repo Repository, sec configSection[S, In]) (settings S, saved bool, at time.Time, err error) {
	if sec.defaults != nil {
		settings = sec.defaults()
	}
	saved, at, err = loadSection(ctx, repo, sec.name, &settings)
	if sec.fill != nil {
		sec.fill(&settings)
	}
	return settings, saved, at, err
}

// readSection returns the settings and the secrets of a section from repo.
func readSection[S, In any](ctx context.Context, s *Service, repo Repository, sec configSection[S, In]) (sectionState[S], error) {
	settings, saved, at, err := readSettings(ctx, repo, sec)
	if err != nil {
		return sectionState[S]{}, err
	}
	secrets, err := s.loadSecrets(ctx, repo, sec.name)
	if err != nil {
		return sectionState[S]{}, err
	}
	return sectionState[S]{settings: settings, saved: saved, at: at, secrets: secrets}, nil
}

// getSection is the read behind the "Get" calls. Administrators only.
func getSection[S, In any](ctx context.Context, s *Service, actor Actor, sec configSection[S, In]) (sectionState[S], error) {
	if err := requirePermission(actor, PermConfigManage); err != nil {
		return sectionState[S]{}, err
	}
	return readSection(ctx, s, s.repo, sec)
}

// updateSection saves a section and records which fields changed (names only, never values)
// in the audit log. Administrators only. It returns the section as stored afterwards.
func updateSection[S, In any](ctx context.Context, s *Service, actor Actor, sec configSection[S, In], in In) (sectionState[S], error) {
	var zero sectionState[S]
	if err := requirePermission(actor, PermConfigManage); err != nil {
		return zero, err
	}
	var out sectionState[S]
	err := s.repo.InTx(ctx, func(tx Repository) error {
		old, err := readSection(ctx, s, tx, sec)
		if err != nil {
			return err
		}
		next, _, err := sec.validate(in, old.secrets)
		if err != nil {
			return err
		}
		changes := changedSettingsFields(old.settings, next)
		data, err := jsonBytes(next)
		if err != nil {
			return err
		}
		if err := tx.PutConfigSection(ctx, sec.name, data, s.nowUTC()); err != nil {
			return err
		}
		typed := sec.typed(in)
		for _, sc := range sec.secrets {
			switch {
			case sc.unusedWhen != nil && sc.unusedWhen(next):
				if !old.secrets.set[sc.name] {
					continue
				}
				if err := tx.DeleteConfigSecret(ctx, sec.name, sc.name); err != nil {
					return err
				}
				changes = append(changes, sc.name)
			case typed[sc.name] != "" && typed[sc.name] != old.secrets.plain[sc.name]: // a typed value equal to the stored one is not a change
				if err := s.saveSecret(ctx, tx, sec.name, sc.name, typed[sc.name]); err != nil {
					return err
				}
				changes = append(changes, sc.name)
			}
		}
		if len(changes) > 0 {
			if err := s.recordAudit(ctx, tx, AuditActorOf(actor), configChangedAudit(sec.name, changes)); err != nil {
				return err
			}
		}
		out, err = readSection(ctx, s, tx, sec)
		return err
	})
	return out, err
}

// changedSettingsFields names the fields (by JSON name, in declaration order) in which two
// settings differ. Values are never listed. It reads the struct itself, so a field added to a
// settings struct is audited without anyone remembering to.
func changedSettingsFields[S any](a, b S) []string {
	va, vb := reflect.ValueOf(a), reflect.ValueOf(b)
	var out []string
	for i := 0; i < va.NumField(); i++ {
		name := settingsFieldName(va.Type().Field(i))
		if name != "" && !reflect.DeepEqual(va.Field(i).Interface(), vb.Field(i).Interface()) {
			out = append(out, name)
		}
	}
	return out
}

func settingsFieldName(f reflect.StructField) string {
	tag, _, _ := strings.Cut(f.Tag.Get("json"), ",")
	switch tag {
	case "-":
		return ""
	case "":
		return f.Name
	}
	return tag
}
