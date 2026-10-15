package objectstore

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/admax12959/X-Project/internal/domain"
)

// These tests need no server: they look at the requests the S3 adapter would send. They guard
// the two things the Chinese cloud providers are picky about (checksums, addressing) in the
// default test set; the behaviour against a real S3 server is in the contract test.

type recorder struct{ reqs []*http.Request }

func (r *recorder) RoundTrip(req *http.Request) (*http.Response, error) {
	// Read the body now: it may be a one-shot reader.
	if req.Body != nil {
		_, _ = io.Copy(io.Discard, req.Body)
	}
	r.reqs = append(r.reqs, req)
	return &http.Response{StatusCode: http.StatusOK, Header: http.Header{}, Body: io.NopCloser(strings.NewReader("")), Request: req}, nil
}

func recordingStore(t *testing.T, mutate func(*domain.ObjectStoreConfig)) (*S3, *recorder) {
	t.Helper()
	cfg := domain.ObjectStoreConfig{
		Driver: domain.StorageS3, Endpoint: "https://oss-cn-hangzhou.example.com", Region: "oss-cn-hangzhou", Bucket: "my-bucket",
		Addressing: domain.AddressingPath, AccessKeyID: "AKID", SecretAccessKey: "secret",
	}
	if mutate != nil {
		mutate(&cfg)
	}
	rec := &recorder{}
	s, err := newS3(cfg, &http.Client{Transport: rec})
	if err != nil {
		t.Fatal(err)
	}
	return s, rec
}

func TestUploadsCarryNoChecksumHeadersOrChunkedEncoding(t *testing.T) {
	t.Parallel()
	for _, scheme := range []string{"https://oss.example.com", "http://minio.internal:9000"} {
		s, rec := recordingStore(t, func(c *domain.ObjectStoreConfig) { c.Endpoint = scheme })
		if err := s.Put(context.Background(), "avatars/1/x.webp", []byte("data"), "image/webp"); err != nil {
			t.Fatalf("%s: Put: %v", scheme, err)
		}
		if len(rec.reqs) != 1 {
			t.Fatalf("%s: requests = %d, want 1", scheme, len(rec.reqs))
		}
		h := rec.reqs[0].Header
		for name := range h {
			l := strings.ToLower(name)
			if strings.HasPrefix(l, "x-amz-checksum") || l == "x-amz-sdk-checksum-algorithm" || l == "x-amz-trailer" {
				t.Errorf("%s: the upload carries %s: %v", scheme, name, h[name])
			}
		}
		if strings.Contains(h.Get("Content-Encoding"), "aws-chunked") || strings.Contains(h.Get("X-Amz-Content-Sha256"), "STREAMING") {
			t.Errorf("%s: the upload is chunk-encoded: %v", scheme, h)
		}
		if h.Get("Content-Type") != "image/webp" {
			t.Errorf("%s: Content-Type = %q", scheme, h.Get("Content-Type"))
		}
	}
}

func TestDownloadsDoNotAskForChecksumValidation(t *testing.T) {
	t.Parallel()
	s, rec := recordingStore(t, nil)
	r, err := s.Open(context.Background(), "k")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	_ = r.Close()
	if v := rec.reqs[0].Header.Get("X-Amz-Checksum-Mode"); v != "" {
		t.Errorf("x-amz-checksum-mode = %q, want none", v)
	}
}

func TestPathStyleAddressingKeepsTheBucketInThePath(t *testing.T) {
	t.Parallel()
	s, rec := recordingStore(t, nil)
	_ = s.Put(context.Background(), "a/b.png", []byte("x"), "")
	u := rec.reqs[0].URL
	if u.Host != "oss-cn-hangzhou.example.com" || u.Path != "/my-bucket/a/b.png" {
		t.Fatalf("url = %s, want https://oss-cn-hangzhou.example.com/my-bucket/a/b.png", u)
	}
}

func TestVirtualHostAddressingPutsTheBucketInTheHostName(t *testing.T) {
	t.Parallel()
	s, rec := recordingStore(t, func(c *domain.ObjectStoreConfig) { c.Addressing = domain.AddressingVirtualHost })
	_ = s.Put(context.Background(), "a/b.png", []byte("x"), "")
	u := rec.reqs[0].URL
	if u.Host != "my-bucket.oss-cn-hangzhou.example.com" || u.Path != "/a/b.png" {
		t.Fatalf("url = %s, want https://my-bucket.oss-cn-hangzhou.example.com/a/b.png", u)
	}
}

func TestListUsesTheOriginalListObjectsCall(t *testing.T) {
	t.Parallel()
	s, rec := recordingStore(t, nil)
	_, _ = s.List(context.Background(), "avatars/")
	q := rec.reqs[0].URL.Query()
	if q.Get("list-type") != "" || q.Get("prefix") != "avatars/" {
		t.Fatalf("query = %v, want a V1 listing with prefix avatars/", q)
	}
}

func TestAnS3ConfigurationNeedsEndpointBucketAndKey(t *testing.T) {
	t.Parallel()
	for name, mutate := range map[string]func(*domain.ObjectStoreConfig){
		"endpoint":   func(c *domain.ObjectStoreConfig) { c.Endpoint = "" },
		"bucket":     func(c *domain.ObjectStoreConfig) { c.Bucket = "" },
		"access key": func(c *domain.ObjectStoreConfig) { c.AccessKeyID = "" },
		"secret key": func(c *domain.ObjectStoreConfig) { c.SecretAccessKey = "" },
	} {
		cfg := domain.ObjectStoreConfig{Driver: domain.StorageS3, Endpoint: "https://e", Bucket: "b", AccessKeyID: "a", SecretAccessKey: "s"}
		mutate(&cfg)
		if _, err := NewS3(cfg); err == nil {
			t.Errorf("missing %s: NewS3 succeeded", name)
		}
	}
}
