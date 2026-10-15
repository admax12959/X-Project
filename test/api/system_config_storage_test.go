package api

import (
	"context"
	"errors"
	"io"
	"net/http"
	"slices"
	"strings"
	"testing"

	"github.com/admax12959/X-Project/internal/apitest"
	"github.com/admax12959/X-Project/internal/app"
	"github.com/admax12959/X-Project/internal/domain"
)

// storageConfigView mirrors GET /api/system-config/storage. There is deliberately no
// access key field: both parts of the key are write-only.
type storageConfigView struct {
	Driver             string `json:"driver"`
	Endpoint           string `json:"endpoint"`
	Region             string `json:"region"`
	Bucket             string `json:"bucket"`
	Addressing         string `json:"addressing"`
	AccessKeyIDSet     bool   `json:"accessKeyIdSet"`
	SecretAccessKeySet bool   `json:"secretAccessKeySet"`
	LocalDir           string `json:"localDir"`
	SecretsUnreadable  bool   `json:"secretsUnreadable"`
	Configured         bool   `json:"configured"`
}

const storageConfigPath = "/api/system-config/storage"

func getStorageConfig(t *testing.T, c *apitest.Client) (storageConfigView, string) {
	t.Helper()
	r := c.Get(storageConfigPath).ExpectStatus(http.StatusOK)
	var v storageConfigView
	r.JSON(&v)
	return v, string(r.Body)
}

// s3Form is a complete, valid S3 form (an Aliyun OSS bucket); over replaces fields.
func s3Form(over map[string]any) map[string]any {
	f := map[string]any{
		"driver": "s3", "endpoint": "https://oss-cn-hangzhou.aliyuncs.com", "region": "oss-cn-hangzhou", "bucket": "xp-avatars",
		"addressing": "virtualHost", "accessKeyId": "AKIDEXAMPLE0001", "secretAccessKey": "s3-secret-key-1",
	}
	for k, v := range over {
		f[k] = v
	}
	return f
}

type connectionTestBody struct {
	OK   bool   `json:"ok"`
	Code string `json:"code"`
}

func TestWithoutConfigurationObjectsLiveOnTheLocalDisk(t *testing.T) {
	t.Parallel()
	env := apitest.New(t)

	v, _ := getStorageConfig(t, env.AdminClient())
	if v.Driver != "local" || v.Configured || v.LocalDir == "" || v.AccessKeyIDSet || v.SecretAccessKeySet {
		t.Fatalf("config = %+v, want the never-saved local default", v)
	}
	if v.Addressing != "path" {
		t.Fatalf("addressing = %q, want the default path", v.Addressing)
	}

	if _, err := env.CurrentObjectStore(context.Background()); err != nil {
		t.Fatalf("CurrentObjectStore: %v", err)
	}
	cfg, ok := env.ObjectStores.LastConfig()
	if !ok || cfg.Driver != domain.StorageLocal || cfg.LocalDir != v.LocalDir || cfg.LocalDir != env.Config.Storage.LocalDir {
		t.Fatalf("store built with %+v, want the local driver on the configured directory %q", cfg, env.Config.Storage.LocalDir)
	}
}

func TestSavedS3ConfigurationIsUsedWithTheDecryptedAccessKey(t *testing.T) {
	t.Parallel()
	env := apitest.New(t)
	admin := env.AdminClient()

	admin.Put(storageConfigPath, s3Form(nil)).ExpectStatus(http.StatusOK)

	// Write-only: the response and a later read say "set", never the value.
	v, body := getStorageConfig(t, admin)
	if v.Driver != "s3" || !v.Configured || v.Endpoint != "https://oss-cn-hangzhou.aliyuncs.com" || v.Region != "oss-cn-hangzhou" ||
		v.Bucket != "xp-avatars" || v.Addressing != "virtualHost" || !v.AccessKeyIDSet || !v.SecretAccessKeySet {
		t.Fatalf("config = %+v", v)
	}
	for _, secret := range []string{"AKIDEXAMPLE0001", "s3-secret-key-1"} {
		if strings.Contains(body, secret) {
			t.Fatalf("the response carries %q: %s", secret, body)
		}
	}
	// Round trip through encryption, and it survives a restart: the store is built with what was saved.
	env.Restart()
	if _, err := env.CurrentObjectStore(context.Background()); err != nil {
		t.Fatalf("CurrentObjectStore: %v", err)
	}
	cfg, _ := env.ObjectStores.LastConfig()
	want := domain.ObjectStoreConfig{
		Driver: domain.StorageS3, LocalDir: env.Config.Storage.LocalDir, Endpoint: "https://oss-cn-hangzhou.aliyuncs.com", Region: "oss-cn-hangzhou", Bucket: "xp-avatars",
		Addressing: domain.AddressingVirtualHost, AccessKeyID: "AKIDEXAMPLE0001", SecretAccessKey: "s3-secret-key-1",
	}
	if cfg != want {
		t.Fatalf("store built with %+v, want %+v", cfg, want)
	}
}

func TestSwitchingBackToLocalDiskKeepsTheStoredAccessKey(t *testing.T) {
	t.Parallel()
	env := apitest.New(t)
	admin := env.AdminClient()
	admin.Put(storageConfigPath, s3Form(nil)).ExpectStatus(http.StatusOK)

	admin.Put(storageConfigPath, map[string]any{"driver": "local"}).ExpectStatus(http.StatusOK)
	if _, err := env.CurrentObjectStore(context.Background()); err != nil {
		t.Fatal(err)
	}
	if cfg, _ := env.ObjectStores.LastConfig(); cfg.Driver != domain.StorageLocal {
		t.Fatalf("store built with %+v, want the local driver", cfg)
	}

	// Back to S3 without typing the key again.
	admin.Put(storageConfigPath, s3Form(map[string]any{"accessKeyId": "", "secretAccessKey": ""})).ExpectStatus(http.StatusOK)
	if _, err := env.CurrentObjectStore(context.Background()); err != nil {
		t.Fatal(err)
	}
	if cfg, _ := env.ObjectStores.LastConfig(); cfg.AccessKeyID != "AKIDEXAMPLE0001" || cfg.SecretAccessKey != "s3-secret-key-1" {
		t.Fatalf("store built with %+v, want the key stored earlier", cfg)
	}
}

func TestLeavingTheAccessKeyEmptyKeepsTheStoredOneAndANewOneReplacesIt(t *testing.T) {
	t.Parallel()
	env := apitest.New(t)
	admin := env.AdminClient()
	admin.Put(storageConfigPath, s3Form(nil)).ExpectStatus(http.StatusOK)

	admin.Put(storageConfigPath, s3Form(map[string]any{"accessKeyId": "", "secretAccessKey": "", "bucket": "xp-other"})).ExpectStatus(http.StatusOK)
	_, _ = env.CurrentObjectStore(context.Background())
	if cfg, _ := env.ObjectStores.LastConfig(); cfg.AccessKeyID != "AKIDEXAMPLE0001" || cfg.SecretAccessKey != "s3-secret-key-1" || cfg.Bucket != "xp-other" {
		t.Fatalf("store built with %+v, want the old key and the new bucket", cfg)
	}

	admin.Put(storageConfigPath, s3Form(map[string]any{"accessKeyId": "", "secretAccessKey": "s3-secret-key-2"})).ExpectStatus(http.StatusOK)
	_, _ = env.CurrentObjectStore(context.Background())
	if cfg, _ := env.ObjectStores.LastConfig(); cfg.AccessKeyID != "AKIDEXAMPLE0001" || cfg.SecretAccessKey != "s3-secret-key-2" {
		t.Fatalf("store built with %+v, want the old key id and the new secret", cfg)
	}
}

func TestSavedStorageConfigurationIsInEffectAtOnceWithoutARestart(t *testing.T) {
	t.Parallel()
	env := apitest.New(t)
	admin := env.AdminClient()
	admin.Put(storageConfigPath, s3Form(nil)).ExpectStatus(http.StatusOK)
	_, _ = env.CurrentObjectStore(context.Background())
	if cfg, _ := env.ObjectStores.LastConfig(); cfg.Driver != domain.StorageS3 {
		t.Fatalf("store built with %+v", cfg)
	}

	admin.Put(storageConfigPath, map[string]any{"driver": "local"}).ExpectStatus(http.StatusOK)
	_, _ = env.CurrentObjectStore(context.Background())
	if cfg, _ := env.ObjectStores.LastConfig(); cfg.Driver != domain.StorageLocal {
		t.Fatalf("store built with %+v, want local straight after the save", cfg)
	}
}

func TestStorageFormIsValidatedLikeTheAdapterNeedsIt(t *testing.T) {
	t.Parallel()
	env := apitest.New(t)
	admin := env.AdminClient()
	cases := []struct {
		name        string
		over        map[string]any
		field, code string
	}{
		{"unknown driver", map[string]any{"driver": "ftp"}, "driver", "invalid"},
		{"endpoint required", map[string]any{"endpoint": ""}, "endpoint", "required"},
		{"endpoint must be http(s)", map[string]any{"endpoint": "ftp://oss.example.com"}, "endpoint", "invalid"},
		{"endpoint needs a host", map[string]any{"endpoint": "https://"}, "endpoint", "invalid"},
		{"endpoint without credentials in it", map[string]any{"endpoint": "https://user:pw@oss.example.com"}, "endpoint", "invalid"},
		{"endpoint without a query", map[string]any{"endpoint": "https://oss.example.com?x=1"}, "endpoint", "invalid"},
		{"bucket required", map[string]any{"bucket": ""}, "bucket", "required"},
		{"bucket name rules", map[string]any{"bucket": "Not_A_Bucket"}, "bucket", "invalid"},
		{"bucket too short", map[string]any{"bucket": "ab"}, "bucket", "invalid"},
		{"unknown addressing", map[string]any{"addressing": "dns"}, "addressing", "invalid"},
		{"region characters", map[string]any{"region": "cn hangzhou"}, "region", "invalid"},
		{"first save needs the key id", map[string]any{"accessKeyId": ""}, "accessKeyId", "required"},
		{"first save needs the secret", map[string]any{"secretAccessKey": ""}, "secretAccessKey", "required"},
		{"key too long", map[string]any{"accessKeyId": strings.Repeat("a", 300)}, "accessKeyId", "tooLong"},
	}
	for _, tc := range cases {
		admin.Put(storageConfigPath, s3Form(tc.over)).ExpectError(http.StatusUnprocessableEntity, tc.field, tc.code)
	}
	if v, _ := getStorageConfig(t, admin); v.Configured || v.Driver != "local" {
		t.Fatalf("a refused save changed the configuration: %+v", v)
	}
	// The local driver needs none of the S3 fields.
	admin.Put(storageConfigPath, map[string]any{"driver": "local"}).ExpectStatus(http.StatusOK)
	// Defaults for what the form leaves out.
	admin.Put(storageConfigPath, s3Form(map[string]any{"region": "", "addressing": ""})).ExpectStatus(http.StatusOK)
	if v, _ := getStorageConfig(t, admin); v.Region != "us-east-1" || v.Addressing != "path" {
		t.Fatalf("defaults = %+v, want us-east-1 and path", v)
	}
}

func TestStorageTestConnectionUsesTheUnsavedFormAndSavesNothing(t *testing.T) {
	t.Parallel()
	env := apitest.New(t)
	admin := env.AdminClient()
	admin.Put(storageConfigPath, s3Form(nil)).ExpectStatus(http.StatusOK)
	before := len(env.ObjectStores.Configs())

	var res connectionTestBody
	admin.Post(storageConfigPath+"/test", s3Form(map[string]any{
		"endpoint": "https://cos.ap-guangzhou.myqcloud.com", "bucket": "typed-1250000000", "secretAccessKey": "typed-not-saved",
	})).ExpectStatus(http.StatusOK).JSON(&res)
	if !res.OK {
		t.Fatalf("test = %+v, want ok", res)
	}
	configs := env.ObjectStores.Configs()
	if len(configs) != before+1 {
		t.Fatalf("stores built = %d, want exactly one more for the test", len(configs)-before)
	}
	if got := configs[len(configs)-1]; got.Endpoint != "https://cos.ap-guangzhou.myqcloud.com" || got.Bucket != "typed-1250000000" ||
		got.SecretAccessKey != "typed-not-saved" || got.AccessKeyID != "AKIDEXAMPLE0001" {
		t.Fatalf("the test connected with %+v, want the unsaved form values", got)
	}
	if v, _ := getStorageConfig(t, admin); v.Bucket != "xp-avatars" || v.Endpoint != "https://oss-cn-hangzhou.aliyuncs.com" {
		t.Fatalf("testing changed the saved configuration: %+v", v)
	}
	if n := len(auditOf(t, admin, "configChanged")); n != 1 {
		t.Fatalf("config audit entries = %d, want only the one from the save", n)
	}
}

func TestStorageTestConnectionOfTheLocalDriverNeedsNoS3Fields(t *testing.T) {
	t.Parallel()
	env := apitest.New(t)

	var res connectionTestBody
	env.AdminClient().Post(storageConfigPath+"/test", map[string]any{"driver": "local"}).ExpectStatus(http.StatusOK).JSON(&res)
	if !res.OK {
		t.Fatalf("test = %+v, want ok", res)
	}
	if cfg, _ := env.ObjectStores.LastConfig(); cfg.Driver != domain.StorageLocal || cfg.LocalDir != env.Config.Storage.LocalDir {
		t.Fatalf("the test used %+v, want the local driver on the configured directory", cfg)
	}
}

func TestStorageTestConnectionFallsBackToTheStoredAccessKeyWhenTheFormLeavesItEmpty(t *testing.T) {
	t.Parallel()
	env := apitest.New(t)
	admin := env.AdminClient()
	admin.Put(storageConfigPath, s3Form(nil)).ExpectStatus(http.StatusOK)

	admin.Post(storageConfigPath+"/test", s3Form(map[string]any{"accessKeyId": "", "secretAccessKey": "", "bucket": "other-bucket"})).ExpectStatus(http.StatusOK)

	if cfg, _ := env.ObjectStores.LastConfig(); cfg.AccessKeyID != "AKIDEXAMPLE0001" || cfg.SecretAccessKey != "s3-secret-key-1" || cfg.Bucket != "other-bucket" {
		t.Fatalf("the test connected with %+v, want the stored key and the typed bucket", cfg)
	}
}

func TestStorageTestConnectionReportsWhatIsWrongAndRejectsABadForm(t *testing.T) {
	t.Parallel()
	env := apitest.New(t)
	admin := env.AdminClient()

	cases := []struct {
		name string
		fail error
		code string
	}{
		{"unreachable", domain.ErrStorageUnreachable, "storageUnreachable"},
		{"access denied", domain.ErrStorageAccessDenied, "storageAccessDenied"},
		{"no such bucket", domain.ErrStorageBucketNotFound, "storageBucketNotFound"},
		{"anything else", errors.New("boom"), "storageFailed"},
	}
	for _, tc := range cases {
		env.Objects.SetFailure(tc.fail)
		var res connectionTestBody
		admin.Post(storageConfigPath+"/test", s3Form(nil)).ExpectStatus(http.StatusOK).JSON(&res)
		if res.OK || res.Code != tc.code {
			t.Errorf("%s: test = %+v, want a failure with code %s", tc.name, res, tc.code)
		}
	}
	env.Objects.SetFailure(nil)

	env.ObjectStores.FailBuild(errors.New("cannot create the directory"))
	var res connectionTestBody
	admin.Post(storageConfigPath+"/test", map[string]any{"driver": "local"}).ExpectStatus(http.StatusOK).JSON(&res)
	if res.OK || res.Code != "storageFailed" {
		t.Errorf("a store that cannot be built: test = %+v, want storageFailed", res)
	}
	env.ObjectStores.FailBuild(nil)

	admin.Post(storageConfigPath+"/test", s3Form(map[string]any{"bucket": ""})).ExpectError(http.StatusUnprocessableEntity, "bucket", "required")
}

func TestOnlyAdministratorsMayReadOrChangeTheStorageConfiguration(t *testing.T) {
	t.Parallel()
	env := apitest.New(t)
	bob := plainUserClient(t, env, "bob")

	bob.Get(storageConfigPath).ExpectError(http.StatusForbidden, "", "forbidden")
	bob.Put(storageConfigPath, s3Form(nil)).ExpectError(http.StatusForbidden, "", "forbidden")
	bob.Post(storageConfigPath+"/test", s3Form(nil)).ExpectError(http.StatusForbidden, "", "forbidden")
	env.Client().Get(storageConfigPath).ExpectError(http.StatusUnauthorized, "", "unauthenticated")

	if v, _ := getStorageConfig(t, env.AdminClient()); v.Configured {
		t.Fatalf("a plain user changed the configuration: %+v", v)
	}
	if n := len(auditOf(t, env.AdminClient(), "configChanged")); n != 0 {
		t.Fatalf("config audit entries = %d, want none", n)
	}
}

func TestSavingStorageConfigurationIsAuditedWithFieldNamesAndNeverValues(t *testing.T) {
	t.Parallel()
	env := apitest.New(t)
	admin := env.AdminClient()

	admin.Put(storageConfigPath, s3Form(nil)).ExpectStatus(http.StatusOK)

	entries := auditOf(t, admin, "configChanged")
	if len(entries) != 1 {
		t.Fatalf("config audit entries = %d, want 1", len(entries))
	}
	e := entries[0]
	if e.Actor.Kind != "user" || e.Actor.Username != apitest.AdminUsername || e.Target.Type != "systemConfig" || e.Target.ID != "storage" || e.Params["section"] != "storage" {
		t.Fatalf("entry = %+v", e)
	}
	var names []string
	for _, n := range e.Params["changes"].([]any) {
		names = append(names, n.(string))
	}
	for _, want := range []string{"driver", "endpoint", "region", "bucket", "addressing", "accessKeyId", "secretAccessKey"} {
		if !slices.Contains(names, want) {
			t.Errorf("changes = %v, missing %s", names, want)
		}
	}
	raw := string(admin.Get(auditPath("action", "configChanged")).Body)
	for _, secret := range []string{"AKIDEXAMPLE0001", "s3-secret-key-1", "aliyuncs.com", "xp-avatars"} {
		if strings.Contains(raw, secret) {
			t.Fatalf("the audit log carries the value %q: %s", secret, raw)
		}
	}

	// Saving the same form again (keys left empty) changes nothing and writes nothing.
	admin.Put(storageConfigPath, s3Form(map[string]any{"accessKeyId": "", "secretAccessKey": ""})).ExpectStatus(http.StatusOK)
	if n := len(auditOf(t, admin, "configChanged")); n != 1 {
		t.Fatalf("config audit entries = %d, want 1 (only the first save)", n)
	}
	// A new secret is listed by name.
	admin.Put(storageConfigPath, s3Form(map[string]any{"secretAccessKey": "s3-secret-key-2"})).ExpectStatus(http.StatusOK)
	if got := auditOf(t, admin, "configChanged")[0].Params["changes"]; len(got.([]any)) != 1 || got.([]any)[0] != "secretAccessKey" {
		t.Fatalf("changes = %v, want [secretAccessKey]", got)
	}
}

func TestAMasterKeyThatChangedLeavesTheStorageKeyUnreadableUntilItIsEnteredAgain(t *testing.T) {
	t.Parallel()
	env := apitest.New(t)
	env.AdminClient().Put(storageConfigPath, s3Form(nil)).ExpectStatus(http.StatusOK)

	env.Restart(func(c *app.Config) { c.SecretKey = "a-different-master-key" })

	// The store cannot be built from garbage: it is unavailable, and the page says why.
	_, err := env.CurrentObjectStore(context.Background())
	if e, ok := domain.AsError(err); !ok || e.Kind != domain.KindUnavailable || e.Errors[0].Code != "storageUnavailable" {
		t.Fatalf("CurrentObjectStore err = %v, want storageUnavailable", err)
	}
	admin := env.AdminClient()
	v, _ := getStorageConfig(t, admin)
	if !v.SecretsUnreadable || !v.AccessKeyIDSet || !v.SecretAccessKeySet {
		t.Fatalf("config = %+v, want the key reported as set but unreadable", v)
	}
	// Saving without re-entering the key cannot work; entering it again fixes it.
	admin.Put(storageConfigPath, s3Form(map[string]any{"accessKeyId": "", "secretAccessKey": ""})).ExpectError(http.StatusUnprocessableEntity, "accessKeyId", "required")
	admin.Put(storageConfigPath, s3Form(nil)).ExpectStatus(http.StatusOK)
	if v, _ := getStorageConfig(t, admin); v.SecretsUnreadable {
		t.Fatalf("config = %+v, want the key readable again", v)
	}
	if _, err := env.CurrentObjectStore(context.Background()); err != nil {
		t.Fatalf("CurrentObjectStore: %v", err)
	}
}

func TestTheStoreTheApplicationUsesIsTheOneTheFactoryBuilt(t *testing.T) {
	t.Parallel()
	env := apitest.New(t)
	store, err := env.CurrentObjectStore(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Put(context.Background(), "avatars/1/x.webp", []byte("pixels"), "image/webp"); err != nil {
		t.Fatal(err)
	}
	r, err := env.Objects.Open(context.Background(), "avatars/1/x.webp")
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	if b, _ := io.ReadAll(r); string(b) != "pixels" {
		t.Fatalf("content = %q", b)
	}
}
