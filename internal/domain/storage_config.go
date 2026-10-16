package domain

import (
	"context"
	"errors"
	"log/slog"
	"net/url"
	"regexp"
	"strings"
	"time"
)

// Object storage configuration (#24, ADR-0007): where the application keeps blobs such as
// avatars. Until an administrator configures an S3-compatible service (Aliyun OSS, Tencent
// COS, MinIO) on the Storage page, objects live in a directory on the local disk. The access
// key (key id and secret) is the secret of the section and is write-only.
//
// As with LDAP, the store is built from the stored configuration whenever it is needed
// (ObjectStore), so a saved change is in effect at once and on every instance. Objects are
// not moved when the configuration changes: the new storage starts empty (for avatars that
// means users show their initials again until they upload a new one).

const (
	storageSecretAccessKeyID     = "accessKeyId"
	storageSecretSecretAccessKey = "secretAccessKey"

	defaultStorageRegion = "us-east-1" // what MinIO and most S3 clones expect when nothing else is said
	maxStorageFieldLen   = 256

	storageTestTimeout = 30 * time.Second
)

var (
	bucketPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9.-]{1,61}[a-z0-9]$`)
	regionPattern = regexp.MustCompile(`^[A-Za-z0-9._-]{1,64}$`)
)

// StorageSettings are the non-secret storage fields. The JSON tags are the stored format and
// the API names.
type StorageSettings struct {
	Driver     ObjectStoreDriver `json:"driver"`
	Endpoint   string            `json:"endpoint"`
	Region     string            `json:"region"`
	Bucket     string            `json:"bucket"`
	Addressing string            `json:"addressing"`
}

// StorageConfigInput is a save or test request. The access key is write-only: empty keeps
// the stored value, anything else replaces it.
type StorageConfigInput struct {
	StorageSettings
	AccessKeyID     string
	SecretAccessKey string
}

// StorageConfigView is what an administrator gets to see. The key is never part of it, only
// whether it is set. SecretsUnreadable means a stored secret cannot be decrypted with the
// current master key (it changed): enter the key again. LocalDir is where the local driver
// keeps its objects (a deployment setting, not editable here).
type StorageConfigView struct {
	StorageSettings
	AccessKeyIDSet     bool
	SecretAccessKeySet bool
	SecretsUnreadable  bool
	LocalDir           string
	Configured         bool // saved at least once; false = defaults shown
	UpdatedAt          time.Time
}

func storageDefaults() StorageSettings {
	return StorageSettings{Driver: StorageLocal, Region: defaultStorageRegion, Addressing: AddressingPath}
}

func (st *StorageSettings) fillDefaults() {
	d := storageDefaults()
	if st.Driver == "" {
		st.Driver = d.Driver
	}
	if strings.TrimSpace(st.Region) == "" {
		st.Region = d.Region
	}
	if strings.TrimSpace(st.Addressing) == "" {
		st.Addressing = d.Addressing
	}
}

// storageSection declares the storage section for the shared skeleton (config_section.go).
var storageSection = configSection[StorageSettings, StorageConfigInput]{
	name: SectionStorage, defaults: storageDefaults, fill: (*StorageSettings).fillDefaults,
	secrets: []sectionSecret[StorageSettings]{{name: storageSecretAccessKeyID}, {name: storageSecretSecretAccessKey}},
	typed: func(in StorageConfigInput) map[string]string {
		return map[string]string{storageSecretAccessKeyID: in.AccessKeyID, storageSecretSecretAccessKey: in.SecretAccessKey}
	},
	validate: func(in StorageConfigInput, stored configSecrets) (StorageSettings, map[string]string, error) {
		v, err := validateStorage(in, stored)
		return v.settings, map[string]string{storageSecretAccessKeyID: v.accessKeyID, storageSecretSecretAccessKey: v.secretAccessKey}, err
	},
}

func (s *Service) readStorage(ctx context.Context, repo Repository) (settings StorageSettings, saved bool, at time.Time, err error) {
	return readSettings(ctx, repo, storageSection)
}

// GetStorageConfig returns the storage configuration for the Storage page. Administrators only.
func (s *Service) GetStorageConfig(ctx context.Context, actor Actor) (StorageConfigView, error) {
	st, err := getSection(ctx, s, actor, storageSection)
	if err != nil {
		return StorageConfigView{}, err
	}
	return s.storageViewOf(st), nil
}

func (s *Service) storageViewOf(st sectionState[StorageSettings]) StorageConfigView {
	return StorageConfigView{
		StorageSettings: st.settings, AccessKeyIDSet: st.secrets.set[storageSecretAccessKeyID],
		SecretAccessKeySet: st.secrets.set[storageSecretSecretAccessKey], SecretsUnreadable: st.secrets.anyUnreadable(),
		LocalDir: s.localDir, Configured: st.saved, UpdatedAt: st.at,
	}
}

// validatedStorage is a storage request that passed validation: the settings to save and the
// access key to use (the typed one, else the stored one).
type validatedStorage struct {
	settings        StorageSettings
	accessKeyID     string
	secretAccessKey string
}

// validateStorage checks a request against the stored state (the stored key counts when the
// request leaves it empty). S3 fields are required only for the S3 driver, but are checked
// for format whenever they are filled in.
func validateStorage(in StorageConfigInput, secrets configSecrets) (validatedStorage, error) {
	var c Collector
	st := in.StorageSettings
	trim := func(field string, p *string) {
		*p = strings.TrimSpace(*p)
		if len([]rune(*p)) > maxStorageFieldLen {
			c.Add(field, CodeTooLong, map[string]any{"max": maxStorageFieldLen})
		}
	}
	trim("endpoint", &st.Endpoint)
	trim("region", &st.Region)
	trim("bucket", &st.Bucket)
	st.Addressing = strings.TrimSpace(st.Addressing)
	st.fillDefaults()
	st.Endpoint = strings.TrimRight(st.Endpoint, "/")
	isS3 := st.Driver == StorageS3

	switch st.Driver {
	case StorageLocal, StorageS3:
	default:
		c.Add("driver", CodeInvalid, map[string]any{"allowed": []string{string(StorageLocal), string(StorageS3)}})
	}
	switch st.Addressing {
	case AddressingPath, AddressingVirtualHost:
	default:
		c.Add("addressing", CodeInvalid, map[string]any{"allowed": []string{AddressingPath, AddressingVirtualHost}})
	}
	switch {
	case st.Endpoint != "":
		if u, err := url.Parse(st.Endpoint); err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Hostname() == "" ||
			u.User != nil || u.RawQuery != "" || u.Fragment != "" {
			c.Add("endpoint", CodeInvalid, map[string]any{"allowed": []string{"http://", "https://"}})
		}
	case isS3:
		c.Add("endpoint", CodeRequired, nil)
	}
	switch {
	case st.Bucket != "":
		if !bucketPattern.MatchString(st.Bucket) {
			c.Add("bucket", CodeInvalid, nil)
		}
	case isS3:
		c.Add("bucket", CodeRequired, nil)
	}
	if !regionPattern.MatchString(st.Region) {
		c.Add("region", CodeInvalid, nil)
	}

	keyID, secret := in.AccessKeyID, in.SecretAccessKey
	if len([]rune(keyID)) > maxStorageFieldLen {
		c.Add(storageSecretAccessKeyID, CodeTooLong, map[string]any{"max": maxStorageFieldLen})
	}
	if len([]rune(secret)) > maxStorageFieldLen {
		c.Add(storageSecretSecretAccessKey, CodeTooLong, map[string]any{"max": maxStorageFieldLen})
	}
	if keyID == "" {
		keyID = secrets.plain[storageSecretAccessKeyID]
	}
	if secret == "" {
		secret = secrets.plain[storageSecretSecretAccessKey]
	}
	if isS3 {
		if keyID == "" {
			c.Add(storageSecretAccessKeyID, CodeRequired, nil)
		}
		if secret == "" {
			c.Add(storageSecretSecretAccessKey, CodeRequired, nil)
		}
	}
	if err := c.Err(); err != nil {
		return validatedStorage{}, err
	}
	return validatedStorage{settings: st, accessKeyID: keyID, secretAccessKey: secret}, nil
}

// UpdateStorageConfig saves the storage configuration and records which fields changed (names
// only) in the audit log. Administrators only. The new configuration is in effect at once.
func (s *Service) UpdateStorageConfig(ctx context.Context, actor Actor, in StorageConfigInput) (StorageConfigView, error) {
	st, err := updateSection(ctx, s, actor, storageSection, in)
	if err != nil {
		return StorageConfigView{}, err
	}
	return s.storageViewOf(st), nil
}

// TestStorageConfig checks that the storage described by the (unsaved) form works: it writes,
// reads back and deletes a probe object. Nothing is saved and nothing is audited. A key left
// empty falls back to the stored one. Administrators only.
func (s *Service) TestStorageConfig(ctx context.Context, actor Actor, in StorageConfigInput) (ConnectionTest, error) {
	if err := requirePermission(actor, PermConfigManage); err != nil {
		return ConnectionTest{}, err
	}
	secrets, err := s.loadSecrets(ctx, s.repo, SectionStorage)
	if err != nil {
		return ConnectionTest{}, err
	}
	v, err := validateStorage(in, secrets)
	if err != nil {
		return ConnectionTest{}, err
	}
	if s.objectStores == nil {
		return ConnectionTest{Code: CodeStorageFailed}, nil
	}
	store, err := s.objectStores.Build(s.objectStoreConfig(v))
	if err != nil {
		slog.Warn("storage: test connection: cannot build the object store", "err", err)
		return ConnectionTest{Code: storageErrorCode(err)}, nil
	}
	ctx, cancel := context.WithTimeout(ctx, storageTestTimeout)
	defer cancel()
	if err := store.TestConnection(ctx); err != nil {
		slog.Warn("storage: test connection failed", "driver", v.settings.Driver, "err", err)
		return ConnectionTest{Code: storageErrorCode(err)}, nil
	}
	return ConnectionTest{OK: true}, nil
}

// storageErrorCode turns an adapter error into the code "test connection" reports.
func storageErrorCode(err error) string {
	switch {
	case errors.Is(err, ErrStorageUnreachable), errors.Is(err, context.DeadlineExceeded):
		return CodeStorageUnreachable
	case errors.Is(err, ErrStorageAccessDenied):
		return CodeStorageAccessDenied
	case errors.Is(err, ErrStorageBucketNotFound):
		return CodeStorageBucketNotFound
	}
	return CodeStorageFailed
}

func (s *Service) objectStoreConfig(v validatedStorage) ObjectStoreConfig {
	cfg := ObjectStoreConfig{Driver: v.settings.Driver, LocalDir: s.localDir}
	if v.settings.Driver == StorageS3 {
		cfg.Endpoint, cfg.Region, cfg.Bucket, cfg.Addressing = v.settings.Endpoint, v.settings.Region, v.settings.Bucket, v.settings.Addressing
		cfg.AccessKeyID, cfg.SecretAccessKey = v.accessKeyID, v.secretAccessKey
	}
	return cfg
}

func storageUnavailable() error { return newKind(KindUnavailable, CodeStorageUnavailable) }

// ObjectStore is the one way the rest of the domain gets the object store (avatars use it):
// built from the stored configuration on every call, the local-disk store while nothing is
// configured. A stored key that cannot be decrypted (the master key changed) or a
// configuration the adapter refuses is reported as an unavailable storage (503
// storageUnavailable) and logged; the administrator fixes it on the Storage page.
func (s *Service) ObjectStore(ctx context.Context) (ObjectStore, error) {
	if s.objectStores == nil {
		return nil, storageUnavailable()
	}
	settings, _, _, err := s.readStorage(ctx, s.repo)
	if err != nil {
		return nil, err
	}
	v := validatedStorage{settings: settings}
	if settings.Driver == StorageS3 {
		secrets, err := s.loadSecrets(ctx, s.repo, SectionStorage)
		if err != nil {
			return nil, err
		}
		if secrets.anyUnreadable() {
			slog.Error("storage: the stored access key cannot be decrypted with the current master key; enter it again on the Storage page")
			return nil, storageUnavailable()
		}
		v.accessKeyID, v.secretAccessKey = secrets.plain[storageSecretAccessKeyID], secrets.plain[storageSecretSecretAccessKey]
	}
	store, err := s.objectStores.Build(s.objectStoreConfig(v))
	if err != nil {
		slog.Error("storage: the stored configuration cannot be used", "err", err)
		return nil, storageUnavailable()
	}
	return store, nil
}
