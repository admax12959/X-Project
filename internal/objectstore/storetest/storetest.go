// Package storetest is the contract every domain.ObjectStore implementation must satisfy.
// The in-memory fake runs it in the default test set (internal/fakes); the local-disk and
// S3-compatible adapters run it under the objectstorecontract build tag (make
// test-storage-contract).
package storetest

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/admax12959/X-Project/internal/domain"
)

// Run runs the contract against stores made by fresh. fresh must return a store that is
// empty and unaffected by the other tests (a new prefix, directory or bucket). Run does
// not clean up: the caller's fresh function owns the lifetime of what it creates.
func Run(t *testing.T, fresh func(t *testing.T) domain.ObjectStore) {
	t.Helper()
	ctx := context.Background()

	t.Run("a put object opens with the same bytes", func(t *testing.T) {
		s := fresh(t)
		want := []byte{0, 1, 2, 255, 254, '\n', 0}
		mustPut(t, s, "avatars/1/a.webp", want, "image/webp")
		if got := mustOpen(t, s, "avatars/1/a.webp"); !bytes.Equal(got, want) {
			t.Fatalf("content = %v, want %v", got, want)
		}
	})

	t.Run("an empty object is allowed", func(t *testing.T) {
		s := fresh(t)
		mustPut(t, s, "empty", nil, "")
		if got := mustOpen(t, s, "empty"); len(got) != 0 {
			t.Fatalf("content = %v, want empty", got)
		}
	})

	t.Run("putting an existing key replaces it", func(t *testing.T) {
		s := fresh(t)
		mustPut(t, s, "k", []byte("first version, longer"), "text/plain")
		mustPut(t, s, "k", []byte("second"), "text/plain")
		if got := mustOpen(t, s, "k"); string(got) != "second" {
			t.Fatalf("content = %q, want the second put", got)
		}
		if infos := mustList(t, s, ""); len(infos) != 1 || infos[0].Size != int64(len("second")) {
			t.Fatalf("list = %+v, want one object of 6 bytes", infos)
		}
	})

	t.Run("opening a missing key is ErrObjectNotFound", func(t *testing.T) {
		s := fresh(t)
		_, err := s.Open(ctx, "no/such/object")
		if !errors.Is(err, domain.ErrObjectNotFound) {
			t.Fatalf("Open err = %v, want ErrObjectNotFound", err)
		}
		mustPut(t, s, "no/such", []byte("x"), "")
		if _, err := s.Open(ctx, "no/such/object"); !errors.Is(err, domain.ErrObjectNotFound) {
			t.Fatalf("Open under an existing object err = %v, want ErrObjectNotFound", err)
		}
	})

	t.Run("delete removes the object and deleting a missing one is fine", func(t *testing.T) {
		s := fresh(t)
		mustPut(t, s, "d/1", []byte("x"), "")
		if err := s.Delete(ctx, "d/1"); err != nil {
			t.Fatalf("Delete: %v", err)
		}
		if _, err := s.Open(ctx, "d/1"); !errors.Is(err, domain.ErrObjectNotFound) {
			t.Fatalf("Open after Delete err = %v, want ErrObjectNotFound", err)
		}
		if err := s.Delete(ctx, "d/1"); err != nil {
			t.Fatalf("Delete of a missing key: %v", err)
		}
		if err := s.Delete(ctx, "never/existed"); err != nil {
			t.Fatalf("Delete of a key never put: %v", err)
		}
	})

	t.Run("list filters by prefix and orders by key", func(t *testing.T) {
		s := fresh(t)
		for _, k := range []string{"avatars/2/b.webp", "avatars/1/z.webp", "avatars/1/a.webp", "other/1", "avatars-old/1"} {
			mustPut(t, s, k, []byte(k), "")
		}
		if got, want := keys(mustList(t, s, "")), []string{"avatars-old/1", "avatars/1/a.webp", "avatars/1/z.webp", "avatars/2/b.webp", "other/1"}; !slices.Equal(got, want) {
			t.Fatalf("List(\"\") = %v, want %v", got, want)
		}
		if got, want := keys(mustList(t, s, "avatars/")), []string{"avatars/1/a.webp", "avatars/1/z.webp", "avatars/2/b.webp"}; !slices.Equal(got, want) {
			t.Fatalf("List(avatars/) = %v, want %v", got, want)
		}
		if got, want := keys(mustList(t, s, "avatars/1/")), []string{"avatars/1/a.webp", "avatars/1/z.webp"}; !slices.Equal(got, want) {
			t.Fatalf("List(avatars/1/) = %v, want %v", got, want)
		}
		// A prefix need not end at a segment boundary.
		if got, want := keys(mustList(t, s, "avatars/1/a")), []string{"avatars/1/a.webp"}; !slices.Equal(got, want) {
			t.Fatalf("List(avatars/1/a) = %v, want %v", got, want)
		}
		if got := mustList(t, s, "nothing/"); len(got) != 0 {
			t.Fatalf("List(nothing/) = %v, want none", got)
		}
	})

	t.Run("list reports size and a modification time", func(t *testing.T) {
		s := fresh(t)
		before := time.Now().Add(-time.Minute)
		mustPut(t, s, "info/x", []byte("12345"), "")
		infos := mustList(t, s, "info/")
		if len(infos) != 1 || infos[0].Key != "info/x" || infos[0].Size != 5 {
			t.Fatalf("list = %+v, want info/x of 5 bytes", infos)
		}
		if mt := infos[0].ModTime; mt.Before(before) || mt.After(time.Now().Add(time.Minute)) {
			t.Fatalf("ModTime = %v, want about now", mt)
		}
	})

	t.Run("keys with spaces, unicode and punctuation work", func(t *testing.T) {
		s := fresh(t)
		names := []string{"头像/用户 1.png", "a b/c+d=e&f", "café/x%20y", "semi;colon,comma/(paren)", "dots.in.name/.hidden"}
		for _, k := range names {
			mustPut(t, s, k, []byte(k), "")
		}
		for _, k := range names {
			if got := mustOpen(t, s, k); string(got) != k {
				t.Fatalf("content of %q = %q", k, got)
			}
		}
		got := keys(mustList(t, s, ""))
		want := slices.Clone(names)
		slices.Sort(want)
		if !slices.Equal(got, want) {
			t.Fatalf("List = %v, want %v", got, want)
		}
	})

	t.Run("invalid keys are rejected by every operation", func(t *testing.T) {
		s := fresh(t)
		for _, k := range []string{"", "/abs", "trailing/", "a//b", "a/./b", "../escape", "a/../../escape", "a/..", "back\\slash", "nul\x00byte", "new\nline", strings.Repeat("k", 513)} {
			if err := s.Put(ctx, k, []byte("x"), ""); !errors.Is(err, domain.ErrInvalidObjectKey) {
				t.Errorf("Put(%q) err = %v, want ErrInvalidObjectKey", k, err)
			}
			if _, err := s.Open(ctx, k); !errors.Is(err, domain.ErrInvalidObjectKey) {
				t.Errorf("Open(%q) err = %v, want ErrInvalidObjectKey", k, err)
			}
			if err := s.Delete(ctx, k); !errors.Is(err, domain.ErrInvalidObjectKey) {
				t.Errorf("Delete(%q) err = %v, want ErrInvalidObjectKey", k, err)
			}
		}
		for _, p := range []string{"/abs", "../x", "a//", "a/../b"} {
			if _, err := s.List(ctx, p); !errors.Is(err, domain.ErrInvalidObjectKey) {
				t.Errorf("List(%q) err = %v, want ErrInvalidObjectKey", p, err)
			}
		}
		if got := mustList(t, s, ""); len(got) != 0 {
			t.Fatalf("invalid keys left objects behind: %v", got)
		}
	})

	t.Run("a long listing is complete", func(t *testing.T) {
		if testing.Short() {
			t.Skip("-short")
		}
		s := fresh(t)
		const n = 1100 // more than one S3 listing page (1000)
		var wg sync.WaitGroup
		jobs := make(chan int)
		errs := make(chan error, n)
		for w := 0; w < 16; w++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for i := range jobs {
					if err := s.Put(ctx, fmt.Sprintf("bulk/%05d", i), []byte{byte(i)}, ""); err != nil {
						errs <- err
					}
				}
			}()
		}
		for i := 0; i < n; i++ {
			jobs <- i
		}
		close(jobs)
		wg.Wait()
		close(errs)
		for err := range errs {
			t.Fatalf("Put: %v", err)
		}
		infos := mustList(t, s, "bulk/")
		if len(infos) != n {
			t.Fatalf("listed %d objects, want %d", len(infos), n)
		}
		if infos[0].Key != "bulk/00000" || infos[n-1].Key != fmt.Sprintf("bulk/%05d", n-1) {
			t.Fatalf("order: first %q last %q", infos[0].Key, infos[n-1].Key)
		}
	})

	t.Run("test connection succeeds and leaves nothing behind", func(t *testing.T) {
		s := fresh(t)
		mustPut(t, s, "keep/me", []byte("x"), "")
		if err := s.TestConnection(ctx); err != nil {
			t.Fatalf("TestConnection: %v", err)
		}
		if got, want := keys(mustList(t, s, "")), []string{"keep/me"}; !slices.Equal(got, want) {
			t.Fatalf("objects after the test = %v, want only %v", got, want)
		}
	})
}

func mustPut(t *testing.T, s domain.ObjectStore, key string, body []byte, contentType string) {
	t.Helper()
	if err := s.Put(context.Background(), key, body, contentType); err != nil {
		t.Fatalf("Put(%q): %v", key, err)
	}
}

func mustOpen(t *testing.T, s domain.ObjectStore, key string) []byte {
	t.Helper()
	r, err := s.Open(context.Background(), key)
	if err != nil {
		t.Fatalf("Open(%q): %v", key, err)
	}
	defer r.Close()
	b, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("read %q: %v", key, err)
	}
	return b
}

func mustList(t *testing.T, s domain.ObjectStore, prefix string) []domain.ObjectInfo {
	t.Helper()
	infos, err := s.List(context.Background(), prefix)
	if err != nil {
		t.Fatalf("List(%q): %v", prefix, err)
	}
	return infos
}

func keys(infos []domain.ObjectInfo) []string {
	out := make([]string, len(infos))
	for i, in := range infos {
		out[i] = in.Key
	}
	return out
}
