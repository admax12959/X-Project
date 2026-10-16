package domain

import (
	"context"
	"errors"
	"testing"
	"time"
)

// noticeRepo is a repository on which mail works and whose mail insert fails with insertErr.
// It classifies the errors the way the MySQL store does: only deadlock marks a transaction as
// lost.
type noticeRepo struct {
	Repository
	insertErr error
}

var errFakeDeadlock = errors.New("fake: deadlock found (1213)")

func (r *noticeRepo) IsTxAborting(err error) bool { return errors.Is(err, errFakeDeadlock) }
func (r *noticeRepo) GetConfigSection(context.Context, string) ([]byte, time.Time, bool, error) {
	return []byte(`{"enabled":true,"host":"smtp.example.com","from":"x@example.com"}`), time.Time{}, true, nil
}
func (r *noticeRepo) ListConfigSecrets(context.Context, string) (map[string]StoredSecret, error) {
	return nil, nil
}
func (r *noticeRepo) GetUser(_ context.Context, id UserID) (User, bool, error) {
	return User{ID: id, Email: "u@example.com"}, true, nil
}
func (r *noticeRepo) InsertMail(context.Context, MailEntry) (int64, error) { return 0, r.insertErr }

func notify(r *noticeRepo) error {
	return NewService(r, Options{}).notifySecurity(context.Background(), r, 7, "u@example.com", MailTemplateSecondFactorEnabled, nil)
}

// A notice that fails as a statement (a constraint, a bad value) leaves the transaction alive, so
// the change it is about may still commit.
func TestASecurityNoticeThatFailsAsAStatementIsSwallowed(t *testing.T) {
	if err := notify(&noticeRepo{insertErr: errors.New("fake: field has no default value (1364)")}); err != nil {
		t.Fatalf("err = %v, want nil", err)
	}
}

// A deadlock rolls the transaction back: swallowing it would let COMMIT succeed on nothing and
// report a credential change that did not happen.
func TestASecurityNoticeThatLostTheTransactionFailsTheChange(t *testing.T) {
	for name, cause := range map[string]error{
		"deadlock":  errFakeDeadlock,
		"cancelled": context.Canceled,
		"deadline":  context.DeadlineExceeded,
	} {
		err := notify(&noticeRepo{insertErr: errors.Join(errors.New("insert mail"), cause)})
		if !errors.Is(err, cause) {
			t.Errorf("%s: err = %v, want it returned", name, err)
		}
	}
}
