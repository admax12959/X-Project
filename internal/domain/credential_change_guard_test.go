package domain

import (
	"context"
	"testing"
)

// A caller that forgot the revoke policy, or asks to keep a session without naming it, is
// refused before any lock is taken. The embedded Repository is nil, so a call to it panics.
func TestChangeCredentialRefusesAnIncompleteRequest(t *testing.T) {
	apply := func(lockedCredential) error { return nil }
	for name, ch := range map[string]credentialChange{
		"no policy":          {User: 7, Apply: apply},
		"others, no session": {User: 7, Apply: apply, Revoke: RevokeOthers},
		"no change":          {User: 7, Revoke: RevokeAll},
	} {
		t.Run(name, func(t *testing.T) {
			var tx struct{ Repository }
			if err := NewService(nil, Options{}).changeCredential(context.Background(), tx, ch); err == nil {
				t.Fatal("want an error")
			}
		})
	}
}
