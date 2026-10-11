package api

import (
	"net/http"
	"sync"
	"testing"
)

// enabledOwnerCount reads the application and the user list and counts the Owners that are enabled.
func (f maintFixture) enabledOwnerCount(t *testing.T) int {
	t.Helper()
	status := map[string]string{}
	for _, u := range listUsers(t, f.admin) {
		status[u.ID] = u.Status
	}
	n := 0
	for _, o := range f.app(t).Owners {
		if status[o.ID] == "enabled" {
			n++
		}
	}
	return n
}

func TestAnApplicationAlwaysKeepsAtLeastOneEnabledOwner(t *testing.T) {
	t.Parallel()
	f := newMaintFixture(t)
	f.putMembers(f.admin, []string{f.owner.ID, f.bob.ID}, []string{}).ExpectStatus(http.StatusOK)
	f.admin.Post("/api/users/"+f.bob.ID+"/disable", nil).ExpectStatus(http.StatusOK) // elarsen remains

	// Handing the application to the one disabled Owner would leave nobody who can act,
	// whoever asks and whether the disabled Owner is new or already listed.
	f.putMembers(f.ownerC, []string{f.bob.ID}, []string{}).ExpectError(http.StatusUnprocessableEntity, "owners", "noEnabledOwner")
	f.putMembers(f.admin, []string{f.bob.ID}, []string{f.pe.ID}).ExpectError(http.StatusUnprocessableEntity, "owners", "noEnabledOwner")

	got := f.app(t)
	if len(got.Owners) != 2 || got.Owners[0].Username != "elarsen" || got.Owners[1].Username != "bob" {
		t.Fatalf("rejected edits changed the owners: %+v", got.Owners)
	}

	// Keeping an enabled Owner next to the disabled one is fine, and so is replacing them.
	f.putMembers(f.ownerC, []string{f.bob.ID, f.owner.ID}, []string{}).ExpectStatus(http.StatusOK)
	f.putMembers(f.ownerC, []string{f.owner.ID}, []string{}).ExpectStatus(http.StatusOK)
	if n := f.enabledOwnerCount(t); n != 1 {
		t.Fatalf("enabled owners = %d, want 1", n)
	}
}

// "Disable X" and "make X the only Owner" run at the same time: whichever commits first,
// the application must not end up with only a disabled Owner.
func TestDisablingAnOwnerWhileTheyAreMadeTheOnlyOwnerNeverLeavesNoEnabledOwner(t *testing.T) {
	t.Parallel()
	f := newMaintFixture(t)
	for i := 0; i < 12; i++ {
		f.admin.Post("/api/users/"+f.bob.ID+"/enable", nil).ExpectStatus(http.StatusOK)
		f.putMembers(f.admin, []string{f.owner.ID, f.bob.ID}, []string{}).ExpectStatus(http.StatusOK)

		start := make(chan struct{})
		var wg sync.WaitGroup
		wg.Add(2)
		go func() {
			defer wg.Done()
			<-start
			if st := f.putMembers(f.ownerC, []string{f.bob.ID}, []string{}).Status; st != http.StatusOK && st != http.StatusUnprocessableEntity {
				t.Errorf("owner update answered %d", st)
			}
		}()
		go func() {
			defer wg.Done()
			<-start
			if st := f.admin.Post("/api/users/"+f.bob.ID+"/disable", nil).Status; st != http.StatusOK && st != http.StatusUnprocessableEntity {
				t.Errorf("disable answered %d", st)
			}
		}()
		close(start)
		wg.Wait()
		if n := f.enabledOwnerCount(t); n == 0 {
			t.Fatalf("round %d: the application has no enabled Owner left", i)
		}
	}
}
