package domain

import (
	"context"
	"errors"
	"io"
	"strings"
	"time"
)

// Object storage port (#24, ADR-0007). The domain keeps blobs (avatars first) in an object
// store and does not care whether it is an S3-compatible bucket (Aliyun OSS, Tencent COS,
// MinIO) or a directory on the local disk. Both adapters satisfy the same contract, run by
// internal/objectstore/storetest against every implementation.

// Errors every ObjectStore implementation returns (wrapped is fine: use errors.Is).
var (
	// ErrObjectNotFound: Open of a key that does not exist.
	ErrObjectNotFound = errors.New("object not found")
	// ErrInvalidObjectKey: the key is not allowed (see ValidObjectKey).
	ErrInvalidObjectKey = errors.New("invalid object key")
	// ErrStorageUnreachable: the storage could not be reached (network, DNS, TLS, timeout) or answered nonsense.
	ErrStorageUnreachable = errors.New("object storage unreachable")
	// ErrStorageAccessDenied: the storage refused the credentials or the operation.
	ErrStorageAccessDenied = errors.New("object storage access denied")
	// ErrStorageBucketNotFound: the configured bucket (or, for local disk, directory) does not exist.
	ErrStorageBucketNotFound = errors.New("object storage bucket not found")
)

// ObjectInfo describes a stored object.
type ObjectInfo struct {
	Key     string
	Size    int64
	ModTime time.Time
}

// ObjectStore stores blobs under slash-separated keys such as "avatars/42/9f2c.webp".
//
// Keys must satisfy ValidObjectKey; every method answers ErrInvalidObjectKey otherwise.
// The store is flat: "directories" exist only as key prefixes.
type ObjectStore interface {
	// Put stores body under key, replacing any existing object. contentType is recorded where
	// the backend supports it (empty = application/octet-stream). Bodies are small (images of
	// at most a few MB), so they are passed whole.
	Put(ctx context.Context, key string, body []byte, contentType string) error
	// Open returns the content of key. The caller closes it. ErrObjectNotFound when missing.
	Open(ctx context.Context, key string) (io.ReadCloser, error)
	// Delete removes key. Deleting a key that does not exist is not an error.
	Delete(ctx context.Context, key string) error
	// List returns every object whose key starts with prefix (empty = all), ordered by key.
	List(ctx context.Context, prefix string) ([]ObjectInfo, error)
	// TestConnection checks that the store works end to end: it writes a probe object, reads
	// it back and deletes it, and leaves nothing behind. It returns one of the ErrStorage*
	// errors (wrapped) saying what is wrong, or another error for anything else.
	TestConnection(ctx context.Context) error
}

// ObjectStoreDriver selects the implementation.
type ObjectStoreDriver string

const (
	StorageLocal ObjectStoreDriver = "local" // a directory on the server's disk; used while no object storage is configured
	StorageS3    ObjectStoreDriver = "s3"    // S3-compatible service: Aliyun OSS, Tencent COS, MinIO
)

// S3 addressing styles. Aliyun OSS and Tencent COS require virtual-host style (the bucket in
// the host name); MinIO usually wants path style (the bucket in the path).
const (
	AddressingPath        = "path"
	AddressingVirtualHost = "virtualHost"
)

// ObjectStoreConfig is everything needed to build an ObjectStore, with the secrets decrypted.
// It is built from the stored configuration (storage_config.go) whenever a store is needed,
// and from the unsaved form for "test connection".
type ObjectStoreConfig struct {
	Driver ObjectStoreDriver

	// LocalDir is the directory of the local driver.
	LocalDir string

	// S3 driver.
	Endpoint        string // https://oss-cn-hangzhou.aliyuncs.com, https://cos.ap-guangzhou.myqcloud.com, http://minio:9000
	Region          string
	Bucket          string
	Addressing      string // AddressingPath or AddressingVirtualHost
	AccessKeyID     string
	SecretAccessKey string
}

// ObjectStoreFactory creates an ObjectStore for a configuration. It does not connect. A nil
// factory in Deps means object storage is not available at all.
type ObjectStoreFactory interface {
	Build(cfg ObjectStoreConfig) (ObjectStore, error)
}

const maxObjectKeyLength = 512

// ValidObjectPrefix reports whether prefix may be passed to List: empty, or a key (optionally
// ending in a slash, optionally cut in the middle of its last segment).
func ValidObjectPrefix(prefix string) bool {
	return prefix == "" || ValidObjectKey(strings.TrimSuffix(prefix, "/"))
}

// ValidObjectKey reports whether key may be used: 1 to 512 bytes of valid UTF-8 without
// control characters or backslashes, made of non-empty segments separated by single slashes,
// where no segment is "." or "..". The rules keep local-disk keys inside their directory and
// mean the same thing on every backend.
func ValidObjectKey(key string) bool {
	if key == "" || len(key) > maxObjectKeyLength {
		return false
	}
	for _, r := range key {
		if r < 0x20 || r == 0x7f || r == '\\' || r == 0xFFFD {
			return false
		}
	}
	for _, seg := range strings.Split(key, "/") {
		if seg == "" || seg == "." || seg == ".." {
			return false
		}
	}
	return true
}
