package objectstore

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/smithy-go"
	smithyhttp "github.com/aws/smithy-go/transport/http"

	"github.com/admax12959/X-Project/internal/domain"
)

// S3 is the S3-compatible adapter: one bucket on Aliyun OSS, Tencent COS, MinIO or any other
// service that speaks the S3 API. It never sets an ACL, so the bucket can (and should) stay
// private with public access blocked (ADR-0007): every read goes through the backend.
//
// Compatibility decisions, all for the Chinese cloud providers:
//   - Request and response checksums are only used when the API requires them. Recent AWS SDKs
//     add a CRC32 trailer / x-amz-checksum header to every upload by default, which OSS and
//     COS reject or mishandle.
//   - Only plain single-object calls are used (PutObject, GetObject, DeleteObject and the
//     original ListObjects), never DeleteObjects (needs a checksum) or ListObjectsV2.
//   - Addressing is configurable: OSS and COS require virtual-host style (bucket in the host
//     name), MinIO normally uses path style.
type S3 struct {
	client *s3.Client
	bucket string
}

var _ domain.ObjectStore = (*S3)(nil)

// NewS3 builds the client. It does not connect.
func NewS3(cfg domain.ObjectStoreConfig) (*S3, error) { return newS3(cfg, nil) }

// newS3 is NewS3 with an optional HTTP client (tests record the requests with it).
func newS3(cfg domain.ObjectStoreConfig, httpClient *http.Client) (*S3, error) {
	switch {
	case cfg.Endpoint == "":
		return nil, errors.New("s3: the endpoint is not configured")
	case cfg.Bucket == "":
		return nil, errors.New("s3: the bucket is not configured")
	case cfg.AccessKeyID == "" || cfg.SecretAccessKey == "":
		return nil, errors.New("s3: the access key is not configured")
	}
	region := cfg.Region
	if region == "" {
		region = "us-east-1"
	}
	if httpClient == nil {
		httpClient = &http.Client{
			Timeout: 60 * time.Second,
			Transport: &http.Transport{
				Proxy:                 http.ProxyFromEnvironment,
				DialContext:           (&net.Dialer{Timeout: 10 * time.Second, KeepAlive: 30 * time.Second}).DialContext,
				TLSHandshakeTimeout:   10 * time.Second,
				ResponseHeaderTimeout: 30 * time.Second,
				MaxIdleConnsPerHost:   4,
				IdleConnTimeout:       60 * time.Second,
			},
		}
	}
	client := s3.New(s3.Options{
		Region:       region,
		BaseEndpoint: aws.String(cfg.Endpoint),
		UsePathStyle: cfg.Addressing != domain.AddressingVirtualHost,
		Credentials:  credentials.NewStaticCredentialsProvider(cfg.AccessKeyID, cfg.SecretAccessKey, ""),

		RequestChecksumCalculation: aws.RequestChecksumCalculationWhenRequired,
		ResponseChecksumValidation: aws.ResponseChecksumValidationWhenRequired,

		RetryMaxAttempts: 2,
		HTTPClient:       httpClient,
	})
	return &S3{client: client, bucket: cfg.Bucket}, nil
}

func (s *S3) Put(ctx context.Context, key string, body []byte, contentType string) error {
	if !domain.ValidObjectKey(key) {
		return domain.ErrInvalidObjectKey
	}
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	_, err := s.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket: &s.bucket, Key: &key, Body: bytes.NewReader(body),
		ContentType: &contentType, ContentLength: aws.Int64(int64(len(body))),
	})
	return classifyS3(err, false)
}

func (s *S3) Open(ctx context.Context, key string) (io.ReadCloser, error) {
	if !domain.ValidObjectKey(key) {
		return nil, domain.ErrInvalidObjectKey
	}
	out, err := s.client.GetObject(ctx, &s3.GetObjectInput{Bucket: &s.bucket, Key: &key})
	if err != nil {
		return nil, classifyS3(err, true)
	}
	return out.Body, nil
}

func (s *S3) Delete(ctx context.Context, key string) error {
	if !domain.ValidObjectKey(key) {
		return domain.ErrInvalidObjectKey
	}
	_, err := s.client.DeleteObject(ctx, &s3.DeleteObjectInput{Bucket: &s.bucket, Key: &key})
	if err = classifyS3(err, true); errors.Is(err, domain.ErrObjectNotFound) {
		return nil
	}
	return err
}

func (s *S3) List(ctx context.Context, prefix string) ([]domain.ObjectInfo, error) {
	if !domain.ValidObjectPrefix(prefix) {
		return nil, domain.ErrInvalidObjectKey
	}
	var out []domain.ObjectInfo
	var marker *string
	for {
		in := &s3.ListObjectsInput{Bucket: &s.bucket, Marker: marker}
		if prefix != "" {
			in.Prefix = &prefix
		}
		page, err := s.client.ListObjects(ctx, in)
		if err != nil {
			return nil, classifyS3(err, false)
		}
		for _, o := range page.Contents {
			info := domain.ObjectInfo{Key: aws.ToString(o.Key), Size: aws.ToInt64(o.Size)}
			if o.LastModified != nil {
				info.ModTime = *o.LastModified
			}
			out = append(out, info)
		}
		if !aws.ToBool(page.IsTruncated) || len(page.Contents) == 0 {
			return out, nil
		}
		// Without a delimiter NextMarker is not returned: continue after the last key.
		marker = page.Contents[len(page.Contents)-1].Key
	}
}

// TestConnection writes, reads back and deletes a probe object, which proves the endpoint,
// the addressing style, the bucket and the credentials together.
func (s *S3) TestConnection(ctx context.Context) error { return probe(ctx, s) }

// classifyS3 maps an SDK error onto the storage errors of the port. keyOp says the request
// named an object: a plain 404 then means "no such object" rather than "no such bucket".
func classifyS3(err error, keyOp bool) error {
	if err == nil {
		return nil
	}
	var api smithy.APIError
	if errors.As(err, &api) {
		switch code := api.ErrorCode(); code {
		case "NoSuchKey":
			return fmt.Errorf("%w: %v", domain.ErrObjectNotFound, err)
		case "NoSuchBucket":
			return fmt.Errorf("%w: %v", domain.ErrStorageBucketNotFound, err)
		case "AccessDenied", "AllAccessDisabled", "InvalidAccessKeyId", "SignatureDoesNotMatch", "InvalidSecurity",
			"AccountProblem", "Forbidden", "InvalidToken", "ExpiredToken", "RequestTimeTooSkewed", "InvalidAccessKeyID",
			"SignatureNotMatch", "UserDisable":
			return fmt.Errorf("%w: %v", domain.ErrStorageAccessDenied, err)
		case "NotFound":
			if keyOp {
				return fmt.Errorf("%w: %v", domain.ErrObjectNotFound, err)
			}
			return fmt.Errorf("%w: %v", domain.ErrStorageBucketNotFound, err)
		}
	}
	var re *smithyhttp.ResponseError
	if errors.As(err, &re) {
		switch status := re.HTTPStatusCode(); {
		case status == http.StatusNotFound && keyOp:
			return fmt.Errorf("%w: %v", domain.ErrObjectNotFound, err)
		case status == http.StatusNotFound:
			return fmt.Errorf("%w: %v", domain.ErrStorageBucketNotFound, err)
		case status == http.StatusUnauthorized || status == http.StatusForbidden:
			return fmt.Errorf("%w: %v", domain.ErrStorageAccessDenied, err)
		case status >= 300 && status < 600 && api != nil:
			return err // the service answered with an error we do not know
		}
	}
	if api != nil {
		return err
	}
	// No answer from the service at all: DNS, connection, TLS, timeout, or something that is
	// not an S3 endpoint.
	return fmt.Errorf("%w: %s", domain.ErrStorageUnreachable, strings.TrimSpace(err.Error()))
}

// Factory builds the real adapters; it implements domain.ObjectStoreFactory.
type Factory struct{}

var _ domain.ObjectStoreFactory = Factory{}

func (Factory) Build(cfg domain.ObjectStoreConfig) (domain.ObjectStore, error) {
	switch cfg.Driver {
	case domain.StorageLocal:
		return NewLocal(cfg.LocalDir)
	case domain.StorageS3:
		return NewS3(cfg)
	}
	return nil, fmt.Errorf("object storage: unknown driver %q", cfg.Driver)
}
