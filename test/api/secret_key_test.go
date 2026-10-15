package api

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/admax12959/X-Project/internal/app"
	"github.com/admax12959/X-Project/internal/apitest"
)

func TestStartupWithoutAMasterKeyFailsAndNamesBothSources(t *testing.T) {
	t.Parallel()
	err := apitest.StartError(t, func(c *app.Config) { c.SecretKey = "" })
	if err == nil {
		t.Fatal("started without a master key")
	}
	for _, want := range []string{"XP_SECRET_KEY", "XP_SECRET_KEY_FILE"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q does not mention %s", err, want)
		}
	}
}

func TestMasterKeyCanComeFromADockerSecretFile(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "xp_secret_key")
	if err := os.WriteFile(path, []byte("key-from-a-docker-secret\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := apitest.StartError(t, func(c *app.Config) { c.SecretKey, c.SecretKeyFile = "", path }); err != nil {
		t.Fatalf("start with a key file: %v", err)
	}
}

func TestMasterKeyFileProblemsStopStartup(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	empty := filepath.Join(dir, "empty")
	if err := os.WriteFile(empty, []byte("\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	full := filepath.Join(dir, "full")
	if err := os.WriteFile(full, []byte("some-key"), 0o600); err != nil {
		t.Fatal(err)
	}
	cases := map[string]func(*app.Config){
		"missing file": func(c *app.Config) { c.SecretKey, c.SecretKeyFile = "", filepath.Join(dir, "nope") },
		"empty file":   func(c *app.Config) { c.SecretKey, c.SecretKeyFile = "", empty },
		"key and file": func(c *app.Config) { c.SecretKeyFile = full }, // SecretKey is still set by the harness
	}
	for name, mutate := range cases {
		if err := apitest.StartError(t, mutate); err == nil {
			t.Errorf("%s: startup succeeded", name)
		}
	}
}
