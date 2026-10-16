package app

import (
	"strings"
	"testing"
)

func TestLoadRefusesValuesTheServiceWouldSilentlyIgnore(t *testing.T) {
	cases := map[string]struct {
		env  map[string]string
		want string // part of the error; "" = Load works
	}{
		"valid":                {map[string]string{"XP_MAIL_RETRY_BACKOFF": "1m,5m", "XP_SECRET_KEY_VERSION": "2"}, ""},
		"negative backoff":     {map[string]string{"XP_MAIL_RETRY_BACKOFF": "1m,-5m"}, "XP_MAIL_RETRY_BACKOFF"},
		"zero key version":     {map[string]string{"XP_SECRET_KEY_VERSION": "0"}, "secret_key_version"},
		"negative key version": {map[string]string{"XP_SECRET_KEY_VERSION": "-1"}, "secret_key_version"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Setenv("XP_DB_DSN", "u:p@tcp(localhost:3306)/x")
			t.Setenv("XP_SECRET_KEY", "k")
			for k, v := range tc.env {
				t.Setenv(k, v)
			}
			_, err := Load()
			switch {
			case tc.want == "" && err != nil:
				t.Fatalf("Load = %v, want success", err)
			case tc.want != "" && (err == nil || !strings.Contains(err.Error(), tc.want)):
				t.Fatalf("Load error = %v, want one naming %q", err, tc.want)
			}
		})
	}
}
