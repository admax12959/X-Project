package fakes

import (
	"bytes"
	"context"
	"io"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/admax12959/X-Project/internal/domain"
)

// ObjectStore is an in-memory domain.ObjectStore, the default in API tests. It honours the
// same contract as the real adapters (internal/objectstore/storetest runs against it).
type ObjectStore struct {
	mu      sync.Mutex
	objects map[string]memObject
	failure error
	now     func() time.Time
}

type memObject struct {
	body        []byte
	contentType string
	modTime     time.Time
}

var _ domain.ObjectStore = (*ObjectStore)(nil)

func NewObjectStore() *ObjectStore {
	return &ObjectStore{objects: map[string]memObject{}, now: time.Now}
}

// SetFailure makes every call return err, like a storage service that went away
// (nil = works again). Use the domain.ErrStorage* errors to simulate a specific failure.
func (s *ObjectStore) SetFailure(err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.failure = err
}

// ContentType returns the content type an object was stored with.
func (s *ObjectStore) ContentType(key string) (string, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	o, ok := s.objects[key]
	return o.contentType, ok
}

func (s *ObjectStore) Put(_ context.Context, key string, body []byte, contentType string) error {
	if !domain.ValidObjectKey(key) {
		return domain.ErrInvalidObjectKey
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.failure != nil {
		return s.failure
	}
	s.objects[key] = memObject{body: bytes.Clone(body), contentType: contentType, modTime: s.now()}
	return nil
}

func (s *ObjectStore) Open(_ context.Context, key string) (io.ReadCloser, error) {
	if !domain.ValidObjectKey(key) {
		return nil, domain.ErrInvalidObjectKey
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.failure != nil {
		return nil, s.failure
	}
	o, ok := s.objects[key]
	if !ok {
		return nil, domain.ErrObjectNotFound
	}
	return io.NopCloser(bytes.NewReader(bytes.Clone(o.body))), nil
}

func (s *ObjectStore) Delete(_ context.Context, key string) error {
	if !domain.ValidObjectKey(key) {
		return domain.ErrInvalidObjectKey
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.failure != nil {
		return s.failure
	}
	delete(s.objects, key)
	return nil
}

func (s *ObjectStore) List(_ context.Context, prefix string) ([]domain.ObjectInfo, error) {
	if !domain.ValidObjectPrefix(prefix) {
		return nil, domain.ErrInvalidObjectKey
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.failure != nil {
		return nil, s.failure
	}
	var out []domain.ObjectInfo
	for k, o := range s.objects {
		if strings.HasPrefix(k, prefix) {
			out = append(out, domain.ObjectInfo{Key: k, Size: int64(len(o.body)), ModTime: o.modTime})
		}
	}
	slices.SortFunc(out, func(a, b domain.ObjectInfo) int { return strings.Compare(a.Key, b.Key) })
	return out, nil
}

func (s *ObjectStore) TestConnection(context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.failure
}

// ObjectStoreFactory is the fake of domain.ObjectStoreFactory: whatever configuration the
// application builds a store for, it returns the one fake ObjectStore, and it records every
// configuration it was asked for (the saved one whenever a store is needed, the unsaved form
// on "test connection"), so tests can assert what the application would have connected to.
type ObjectStoreFactory struct {
	store *ObjectStore

	mu       sync.Mutex
	configs  []domain.ObjectStoreConfig
	buildErr error
}

var _ domain.ObjectStoreFactory = (*ObjectStoreFactory)(nil)

func NewObjectStoreFactory(store *ObjectStore) *ObjectStoreFactory {
	return &ObjectStoreFactory{store: store}
}

func (f *ObjectStoreFactory) Build(cfg domain.ObjectStoreConfig) (domain.ObjectStore, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.configs = append(f.configs, cfg)
	if f.buildErr != nil {
		return nil, f.buildErr
	}
	return f.store, nil
}

// Configs returns every configuration a store was built for, oldest first.
func (f *ObjectStoreFactory) Configs() []domain.ObjectStoreConfig {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]domain.ObjectStoreConfig(nil), f.configs...)
}

// LastConfig returns the configuration of the latest Build call.
func (f *ObjectStoreFactory) LastConfig() (domain.ObjectStoreConfig, bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.configs) == 0 {
		return domain.ObjectStoreConfig{}, false
	}
	return f.configs[len(f.configs)-1], true
}

// FailBuild makes Build return err (nil = works again), like an adapter refusing a configuration.
func (f *ObjectStoreFactory) FailBuild(err error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.buildErr = err
}
