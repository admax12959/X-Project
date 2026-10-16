package domain

import (
	"context"
	"encoding/json"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"
)

// configRepo is the part of the repository a system configuration save touches, kept in maps.
type configRepo struct {
	Repository
	sections   map[string][]byte
	secrets    map[string]map[string]StoredSecret
	secretPuts int // PutConfigSecret calls: a secret that is not stored again does not count
	audits     []StoredAuditEntry
}

func newConfigRepo() *configRepo {
	return &configRepo{sections: map[string][]byte{}, secrets: map[string]map[string]StoredSecret{}}
}

func (r *configRepo) InTx(_ context.Context, fn func(Repository) error) error { return fn(r) }
func (r *configRepo) GetConfigSection(_ context.Context, section string) ([]byte, time.Time, bool, error) {
	data, ok := r.sections[section]
	return data, time.Time{}, ok, nil
}
func (r *configRepo) PutConfigSection(_ context.Context, section string, data []byte, _ time.Time) error {
	r.sections[section] = data
	return nil
}
func (r *configRepo) ListConfigSecrets(_ context.Context, section string) (map[string]StoredSecret, error) {
	out := map[string]StoredSecret{}
	for k, v := range r.secrets[section] {
		out[k] = v
	}
	return out, nil
}
func (r *configRepo) PutConfigSecret(_ context.Context, section, field string, sec StoredSecret, _ time.Time) error {
	if r.secrets[section] == nil {
		r.secrets[section] = map[string]StoredSecret{}
	}
	r.secrets[section][field] = sec
	r.secretPuts++
	return nil
}
func (r *configRepo) DeleteConfigSecret(_ context.Context, section, field string) error {
	delete(r.secrets[section], field)
	return nil
}
func (r *configRepo) AppendAudit(_ context.Context, e StoredAuditEntry) error {
	r.audits = append(r.audits, e)
	return nil
}

func configAdmin() Actor { return Actor{User: User{ID: 1, IsAdmin: true}} }

func newConfigService(repo Repository) *Service {
	return NewService(repo, Options{SecretKey: "a-master-key-for-tests"})
}

// lastChanges returns the field names of the newest config audit entry and checks that it is
// about the section and holds nothing but names.
func lastChanges(t *testing.T, repo *configRepo, section string) []string {
	t.Helper()
	if len(repo.audits) == 0 {
		t.Fatal("no audit entry")
	}
	e := repo.audits[len(repo.audits)-1]
	var p struct {
		Section string   `json:"section"`
		Changes []string `json:"changes"`
	}
	if err := json.Unmarshal(e.Params, &p); err != nil {
		t.Fatal(err)
	}
	if e.Action != AuditConfigChanged || e.TargetID != section || p.Section != section {
		t.Fatalf("audit entry = %+v, want config.changed of %s", e, section)
	}
	return p.Changes
}

func ldapInput(password string, mutate func(*LDAPSettings)) LDAPConfigInput {
	in := LDAPConfigInput{BindPassword: password, LDAPSettings: LDAPSettings{
		Enabled: true, URL: "ldaps://ldap.corp.example:636", BindDN: "cn=svc,dc=corp,dc=example", BaseDN: "dc=corp,dc=example",
	}}
	if mutate != nil {
		mutate(&in.LDAPSettings)
	}
	return in
}

func TestTheAuditDiffComesFromTheFieldsOfTheSettings(t *testing.T) {
	type toy struct {
		Name    string `json:"name"`
		Count   int    `json:"count"`
		Skipped string `json:"-"`
		Newer   bool   `json:"newer,omitempty"`
		Untaged string
	}
	got := changedSettingsFields(toy{}, toy{Name: "a", Count: 1, Skipped: "x", Newer: true, Untaged: "y"})
	if want := []string{"name", "count", "newer", "Untaged"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("changes = %v, want %v (declaration order, JSON names, no skipped field)", got, want)
	}
	if got := changedSettingsFields(toy{Name: "a"}, toy{Name: "a"}); len(got) != 0 {
		t.Fatalf("changes of equal settings = %v", got)
	}
}

func TestEveryFieldOfEverySectionIsInTheAuditDiff(t *testing.T) {
	flip := func(v reflect.Value) {
		switch v.Kind() {
		case reflect.Bool:
			v.SetBool(true)
		case reflect.String:
			v.SetString("changed")
		case reflect.Int:
			v.SetInt(7)
		default:
			t.Fatalf("a settings field of kind %s: teach this test to change it", v.Kind())
		}
	}
	for _, settings := range []any{LDAPSettings{}, SMTPSettings{}, StorageSettings{}} {
		typ := reflect.TypeOf(settings)
		changed := reflect.New(typ).Elem()
		for i := 0; i < typ.NumField(); i++ {
			flip(changed.Field(i))
		}
		var got []string
		switch v := changed.Interface().(type) {
		case LDAPSettings:
			got = changedSettingsFields(LDAPSettings{}, v)
		case SMTPSettings:
			got = changedSettingsFields(SMTPSettings{}, v)
		case StorageSettings:
			got = changedSettingsFields(StorageSettings{}, v)
		}
		if len(got) != typ.NumField() {
			t.Errorf("%s: %d of %d fields in the diff: %v", typ.Name(), len(got), typ.NumField(), got)
		}
	}
}

func TestASavedSectionAuditsTheChangedFieldsAndSecretNamesNeverValues(t *testing.T) {
	repo := newConfigRepo()
	svc := newConfigService(repo)
	ctx := context.Background()

	if _, err := svc.UpdateLDAPConfig(ctx, configAdmin(), ldapInput("pw-one-1", nil)); err != nil {
		t.Fatal(err)
	}
	got := lastChanges(t, repo, SectionLDAP)
	for _, want := range []string{"enabled", "url", "bindDn", "baseDn", "bindPassword"} {
		if !slices.Contains(got, want) {
			t.Errorf("first save: changes %v lack %s", got, want)
		}
	}
	for _, e := range repo.audits {
		if strings.Contains(string(e.Params), "pw-one-1") || strings.Contains(string(e.Params), "ldap.corp.example") {
			t.Fatalf("audit params hold a value: %s", e.Params)
		}
	}

	// A second save that changes one field and leaves the password empty: that field only.
	view, err := svc.UpdateLDAPConfig(ctx, configAdmin(), ldapInput("", func(l *LDAPSettings) { l.TimeoutSeconds = 30 }))
	if err != nil {
		t.Fatal(err)
	}
	if got := lastChanges(t, repo, SectionLDAP); !reflect.DeepEqual(got, []string{"timeoutSeconds"}) {
		t.Fatalf("changes = %v, want [timeoutSeconds]", got)
	}
	if !view.BindPasswordSet || view.TimeoutSeconds != 30 || !view.Configured {
		t.Fatalf("view = %+v, want the saved settings and a password that is set", view)
	}
	if repo.secretPuts != 1 {
		t.Fatalf("secret stored %d times, want once: an empty password keeps the stored one", repo.secretPuts)
	}

	// Nothing changed: nothing audited.
	n := len(repo.audits)
	if _, err := svc.UpdateLDAPConfig(ctx, configAdmin(), ldapInput("", func(l *LDAPSettings) { l.TimeoutSeconds = 30 })); err != nil {
		t.Fatal(err)
	}
	if len(repo.audits) != n {
		t.Fatalf("a save without changes wrote an audit entry: %s", repo.audits[len(repo.audits)-1].Params)
	}
}

func TestASecretTheSettingsNoLongerUseIsDeletedAndAudited(t *testing.T) {
	repo := newConfigRepo()
	svc := newConfigService(repo)
	ctx := context.Background()
	if _, err := svc.UpdateLDAPConfig(ctx, configAdmin(), ldapInput("pw-one-1", nil)); err != nil {
		t.Fatal(err)
	}
	view, err := svc.UpdateLDAPConfig(ctx, configAdmin(), ldapInput("", func(l *LDAPSettings) { l.BindDN = "" }))
	if err != nil {
		t.Fatal(err)
	}
	if view.BindPasswordSet || len(repo.secrets[SectionLDAP]) != 0 {
		t.Fatalf("password still stored: view %+v, rows %v", view, repo.secrets)
	}
	if got := lastChanges(t, repo, SectionLDAP); !reflect.DeepEqual(got, []string{"bindDn", "bindPassword"}) {
		t.Fatalf("changes = %v, want [bindDn bindPassword]", got)
	}
}

func TestOnlyAnAdministratorSavesASection(t *testing.T) {
	repo := newConfigRepo()
	svc := newConfigService(repo)
	if _, err := svc.UpdateLDAPConfig(context.Background(), Actor{User: User{ID: 2}}, ldapInput("pw-one-1", nil)); err == nil {
		t.Fatal("a non-administrator saved the LDAP configuration")
	}
	if len(repo.sections) != 0 || len(repo.audits) != 0 {
		t.Fatalf("something was written: %v, %d audits", repo.sections, len(repo.audits))
	}
}

func smtpInput(password string, mutate func(*SMTPSettings)) SMTPConfigInput {
	in := SMTPConfigInput{Password: password, SMTPSettings: SMTPSettings{
		Enabled: true, Host: "smtp.corp.example", Port: 587, Security: MailSecurityStartTLS, Username: "mailer",
		From: "noreply@corp.example", SiteURL: "https://xp.corp.example",
	}}
	if mutate != nil {
		mutate(&in.SMTPSettings)
	}
	return in
}

func storageInput(keyID, secret string) StorageConfigInput {
	return storageInputAt("https://s3.corp.example", keyID, secret)
}

func storageInputAt(endpoint, keyID, secret string) StorageConfigInput {
	return StorageConfigInput{AccessKeyID: keyID, SecretAccessKey: secret, StorageSettings: StorageSettings{
		Driver: StorageS3, Endpoint: endpoint, Bucket: "xp-objects",
	}}
}

// A secret typed again with the value that is already stored is not a change: it is not
// encrypted and stored again and not audited, whatever the section.
func TestATypedSecretEqualToTheStoredOneIsNotAChange(t *testing.T) {
	ctx := context.Background()
	for _, tc := range []struct {
		name    string
		section string
		save    func(*Service) error // saves the same form every time, with the secrets typed
		puts    int                  // secrets stored by the first save
	}{
		{"ldap", SectionLDAP, func(s *Service) error {
			_, err := s.UpdateLDAPConfig(ctx, configAdmin(), ldapInput("pw-one-1", nil))
			return err
		}, 1},
		{"smtp", SectionSMTP, func(s *Service) error {
			_, err := s.UpdateSMTPConfig(ctx, configAdmin(), smtpInput("pw-one-1", nil))
			return err
		}, 1},
		{"storage", SectionStorage, func(s *Service) error {
			_, err := s.UpdateStorageConfig(ctx, configAdmin(), storageInput("key-id-1", "secret-1"))
			return err
		}, 2},
	} {
		t.Run(tc.name, func(t *testing.T) {
			repo := newConfigRepo()
			svc := newConfigService(repo)
			if err := tc.save(svc); err != nil {
				t.Fatal(err)
			}
			if repo.secretPuts != tc.puts {
				t.Fatalf("first save stored %d secrets, want %d", repo.secretPuts, tc.puts)
			}
			audits := len(repo.audits)
			if err := tc.save(svc); err != nil {
				t.Fatal(err)
			}
			if repo.secretPuts != tc.puts || len(repo.audits) != audits {
				t.Fatalf("same form saved again: %d secrets stored (want %d), %d audit entries (want %d)",
					repo.secretPuts, tc.puts, len(repo.audits), audits)
			}
		})
	}
}

// snapshotRepo plays the isolation of a MySQL transaction (REPEATABLE READ) over a configRepo:
// reads inside InTx see the state as of its start, reads outside see the live state. onRead is
// a concurrent save to the live state, run once right after the first settings read.
type snapshotRepo struct {
	*configRepo
	onRead func()
	fired  *bool
	inTx   bool
}

func (r *snapshotRepo) InTx(_ context.Context, fn func(Repository) error) error {
	if r.inTx {
		return fn(r)
	}
	snap := newConfigRepo()
	for k, v := range r.sections {
		snap.sections[k] = v
	}
	for section, fields := range r.secrets {
		snap.secrets[section] = map[string]StoredSecret{}
		for k, v := range fields {
			snap.secrets[section][k] = v
		}
	}
	return fn(&snapshotRepo{configRepo: snap, onRead: r.onRead, fired: r.fired, inTx: true})
}

func (r *snapshotRepo) GetConfigSection(ctx context.Context, section string) ([]byte, time.Time, bool, error) {
	data, at, ok, err := r.configRepo.GetConfigSection(ctx, section)
	if r.onRead != nil && !*r.fired {
		*r.fired = true
		r.onRead()
	}
	return data, at, ok, err
}

type captureStores struct{ cfg ObjectStoreConfig }

func (c *captureStores) Build(cfg ObjectStoreConfig) (ObjectStore, error) {
	c.cfg = cfg
	return nil, nil
}

type captureDirectories struct{ cfg LDAPConfig }

func (c *captureDirectories) Build(cfg LDAPConfig) (Directory, error) {
	c.cfg = cfg
	return nil, nil
}

// The settings and the secrets of a section are read in one transaction: a save that lands
// between the two reads must not pair the old endpoint with the new credentials (#40 item 4).
func TestTheObjectStoreIsBuiltFromOneStateOfTheSettingsAndTheKey(t *testing.T) {
	ctx := context.Background()
	live := newConfigRepo()
	writer := newConfigService(live)
	if _, err := writer.UpdateStorageConfig(ctx, configAdmin(), storageInputAt("https://old.corp.example", "old-key", "old-secret")); err != nil {
		t.Fatal(err)
	}
	repo := &snapshotRepo{configRepo: live, fired: new(bool)}
	repo.onRead = func() {
		if _, err := writer.UpdateStorageConfig(ctx, configAdmin(), storageInputAt("https://new.corp.example", "new-key", "new-secret")); err != nil {
			t.Error(err)
		}
	}
	stores := &captureStores{}
	svc := NewService(repo, Options{SecretKey: "a-master-key-for-tests", ObjectStoreFactory: stores})
	if _, err := svc.ObjectStore(ctx); err != nil {
		t.Fatal(err)
	}
	if !*repo.fired {
		t.Fatal("the concurrent save never ran")
	}
	got := [3]string{stores.cfg.Endpoint, stores.cfg.AccessKeyID, stores.cfg.SecretAccessKey}
	if got != [3]string{"https://old.corp.example", "old-key", "old-secret"} && got != [3]string{"https://new.corp.example", "new-key", "new-secret"} {
		t.Fatalf("object store built from %v: the endpoint and the key come from different saves", got)
	}
}

func TestTheDirectoryIsBuiltFromOneStateOfTheSettingsAndThePassword(t *testing.T) {
	ctx := context.Background()
	live := newConfigRepo()
	writer := newConfigService(live)
	if _, err := writer.UpdateLDAPConfig(ctx, configAdmin(), ldapInput("old-pw", func(l *LDAPSettings) { l.URL = "ldaps://old.corp.example" })); err != nil {
		t.Fatal(err)
	}
	repo := &snapshotRepo{configRepo: live, fired: new(bool)}
	repo.onRead = func() {
		if _, err := writer.UpdateLDAPConfig(ctx, configAdmin(), ldapInput("new-pw", func(l *LDAPSettings) { l.URL = "ldaps://new.corp.example" })); err != nil {
			t.Error(err)
		}
	}
	dirs := &captureDirectories{}
	svc := NewService(repo, Options{SecretKey: "a-master-key-for-tests", DirectoryFactory: dirs})
	if _, err := svc.currentDirectory(ctx); err != nil {
		t.Fatal(err)
	}
	if !*repo.fired {
		t.Fatal("the concurrent save never ran")
	}
	got := [2]string{dirs.cfg.URL, dirs.cfg.BindPassword}
	if got != [2]string{"ldaps://old.corp.example", "old-pw"} && got != [2]string{"ldaps://new.corp.example", "new-pw"} {
		t.Fatalf("directory built from %v: the URL and the password come from different saves", got)
	}
}
