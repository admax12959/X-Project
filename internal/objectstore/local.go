// Package objectstore holds the real adapters of domain.ObjectStore: a directory on the local
// disk (used while no object storage is configured) and an S3-compatible client for Aliyun
// OSS, Tencent COS and MinIO. Both run the shared contract in storetest.
package objectstore

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"syscall"

	"github.com/admax12959/X-Project/internal/domain"
)

// tempPrefix marks files that are being written; List never shows them.
const tempPrefix = ".xp-tmp-"

// Local stores objects as files below one directory. A key is a relative path; ValidObjectKey
// guarantees it cannot leave the directory. Writes go to a temporary file and are renamed
// into place, so a reader never sees a half-written object.
//
// A key cannot be both an object and the prefix of another key ("a" and "a/b"), which a
// bucket allows; the keys this application uses (avatars/<user>/<hash>) never do that.
type Local struct {
	root string
}

var _ domain.ObjectStore = (*Local)(nil)

// NewLocal creates the directory if needed and returns a store rooted there.
func NewLocal(dir string) (*Local, error) {
	if strings.TrimSpace(dir) == "" {
		return nil, errors.New("local object storage: the directory is not configured (storage_dir / XP_STORAGE_DIR)")
	}
	root, err := filepath.Abs(dir)
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(root, 0o750); err != nil {
		return nil, classifyFS(err)
	}
	return &Local{root: root}, nil
}

func (l *Local) path(key string) string { return filepath.Join(l.root, filepath.FromSlash(key)) }

func (l *Local) Put(_ context.Context, key string, body []byte, _ string) error {
	if !domain.ValidObjectKey(key) {
		return domain.ErrInvalidObjectKey
	}
	dst := l.path(key)
	if err := os.MkdirAll(filepath.Dir(dst), 0o750); err != nil {
		return classifyFS(err)
	}
	tmp, err := os.CreateTemp(filepath.Dir(dst), tempPrefix+"*")
	if err != nil {
		return classifyFS(err)
	}
	_, werr := io.Copy(tmp, bytes.NewReader(body))
	if werr == nil {
		werr = tmp.Sync()
	}
	if cerr := tmp.Close(); werr == nil {
		werr = cerr
	}
	if werr == nil {
		werr = os.Chmod(tmp.Name(), 0o640)
	}
	if werr == nil {
		werr = os.Rename(tmp.Name(), dst)
	}
	if werr != nil {
		_ = os.Remove(tmp.Name())
		return classifyFS(werr)
	}
	return nil
}

func (l *Local) Open(_ context.Context, key string) (io.ReadCloser, error) {
	if !domain.ValidObjectKey(key) {
		return nil, domain.ErrInvalidObjectKey
	}
	f, err := os.Open(l.path(key))
	if err != nil {
		if isNotFound(err) {
			return nil, domain.ErrObjectNotFound
		}
		return nil, classifyFS(err)
	}
	if st, err := f.Stat(); err != nil || st.IsDir() { // a prefix, not an object
		_ = f.Close()
		return nil, domain.ErrObjectNotFound
	}
	return f, nil
}

func (l *Local) Delete(_ context.Context, key string) error {
	if !domain.ValidObjectKey(key) {
		return domain.ErrInvalidObjectKey
	}
	p := l.path(key)
	if st, err := os.Lstat(p); err == nil && st.IsDir() {
		return nil // a prefix, not an object
	}
	if err := os.Remove(p); err != nil {
		if isNotFound(err) {
			return nil
		}
		return classifyFS(err)
	}
	// Tidy: remove the directories the object leaves empty (never the root). Best effort.
	for d := filepath.Dir(p); d != l.root && strings.HasPrefix(d, l.root); d = filepath.Dir(d) {
		if os.Remove(d) != nil { // not empty or not ours to remove
			break
		}
	}
	return nil
}

func (l *Local) List(ctx context.Context, prefix string) ([]domain.ObjectInfo, error) {
	if !domain.ValidObjectPrefix(prefix) {
		return nil, domain.ErrInvalidObjectKey
	}
	var out []domain.ObjectInfo
	err := filepath.WalkDir(l.root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			if isNotFound(err) {
				return nil
			}
			return err
		}
		if cerr := ctx.Err(); cerr != nil {
			return cerr
		}
		if d.IsDir() || strings.HasPrefix(d.Name(), tempPrefix) || !d.Type().IsRegular() {
			return nil
		}
		rel, err := filepath.Rel(l.root, p)
		if err != nil {
			return err
		}
		key := filepath.ToSlash(rel)
		if !strings.HasPrefix(key, prefix) {
			return nil
		}
		st, err := d.Info()
		if err != nil {
			if isNotFound(err) {
				return nil // deleted while listing
			}
			return err
		}
		out = append(out, domain.ObjectInfo{Key: key, Size: st.Size(), ModTime: st.ModTime()})
		return nil
	})
	if err != nil {
		return nil, classifyFS(err)
	}
	slices.SortFunc(out, func(a, b domain.ObjectInfo) int { return strings.Compare(a.Key, b.Key) })
	return out, nil
}

// TestConnection writes, reads back and deletes a probe object below the directory.
func (l *Local) TestConnection(ctx context.Context) error {
	return probe(ctx, l)
}

// probe is the end-to-end check shared by the adapters.
func probe(ctx context.Context, s domain.ObjectStore) error {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		return err
	}
	key := ".xp-connection-test/" + hex.EncodeToString(b)
	want := []byte("x-project connection test " + hex.EncodeToString(b))
	if err := s.Put(ctx, key, want, "text/plain"); err != nil {
		return fmt.Errorf("write: %w", err)
	}
	r, err := s.Open(ctx, key)
	if err != nil {
		_ = s.Delete(ctx, key)
		return fmt.Errorf("read: %w", err)
	}
	got, err := io.ReadAll(r)
	_ = r.Close()
	if err != nil {
		_ = s.Delete(ctx, key)
		return fmt.Errorf("read: %w", err)
	}
	if err := s.Delete(ctx, key); err != nil {
		return fmt.Errorf("delete: %w", err)
	}
	if !bytes.Equal(got, want) {
		return fmt.Errorf("read: %w: the probe object came back different", domain.ErrStorageUnreachable)
	}
	return nil
}

func isNotFound(err error) bool {
	return errors.Is(err, fs.ErrNotExist) || errors.Is(err, syscall.ENOTDIR)
}

// classifyFS maps file system errors onto the storage errors of the port.
func classifyFS(err error) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, fs.ErrPermission), errors.Is(err, syscall.EROFS):
		return fmt.Errorf("%w: %v", domain.ErrStorageAccessDenied, err)
	case errors.Is(err, fs.ErrNotExist):
		return fmt.Errorf("%w: %v", domain.ErrStorageBucketNotFound, err)
	}
	return err
}
