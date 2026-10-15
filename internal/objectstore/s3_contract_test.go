//go:build objectstorecontract

// Contract tests of the S3-compatible adapter against a real MinIO server. Not part of the
// default test set; run with:
//
//	make test-storage-contract
//
// By default a throwaway MinIO container is started (unique name, dynamic host port, label
// xproj-test). The community MinIO images were removed from Docker Hub and Quay in 2025/26;
// the default image is Chainguard's build (cgr.dev/chainguard/minio), override it with
// XP_TEST_MINIO_IMAGE (an image whose entrypoint is `minio` or that runs `minio server`
// given as arguments, e.g. pgsty/minio). To use an existing S3-compatible server instead, set
// XP_TEST_S3_ENDPOINT, XP_TEST_S3_ACCESS_KEY and XP_TEST_S3_SECRET_KEY (and optionally
// XP_TEST_S3_REGION, XP_TEST_S3_ADDRESSING); each test creates a bucket of its own and
// removes it again. The same tests can be pointed at Aliyun OSS or Tencent COS that way
// (the account needs permission to create and delete buckets).
package objectstore_test

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"

	"github.com/admax12959/X-Project/internal/domain"
	"github.com/admax12959/X-Project/internal/objectstore"
	"github.com/admax12959/X-Project/internal/objectstore/storetest"
)

const (
	defaultImage = "cgr.dev/chainguard/minio:latest"
	rootUser     = "xp-contract-access"
	rootPassword = "xp-contract-secret-key"
)

var server struct {
	endpoint, region, addressing, accessKey, secretKey string
}

func TestMain(m *testing.M) {
	code := func() int {
		if e := os.Getenv("XP_TEST_S3_ENDPOINT"); e != "" {
			server.endpoint, server.region, server.addressing = e, os.Getenv("XP_TEST_S3_REGION"), os.Getenv("XP_TEST_S3_ADDRESSING")
			server.accessKey, server.secretKey = os.Getenv("XP_TEST_S3_ACCESS_KEY"), os.Getenv("XP_TEST_S3_SECRET_KEY")
		} else {
			stop, err := startMinIO()
			if err != nil {
				fmt.Fprintln(os.Stderr, "s3 contract: start MinIO:", err)
				return 1
			}
			defer stop()
		}
		if server.region == "" {
			server.region = "us-east-1"
		}
		if server.addressing == "" {
			server.addressing = domain.AddressingPath
		}
		return m.Run()
	}()
	os.Exit(code)
}

func startMinIO() (stop func(), err error) {
	b := make([]byte, 4)
	_, _ = rand.Read(b)
	name := "xproj-test-minio-" + hex.EncodeToString(b)
	image := os.Getenv("XP_TEST_MINIO_IMAGE")
	if image == "" {
		image = defaultImage
	}
	args := []string{"run", "-d", "--rm", "--name", name, "--label", "xproj-test=1",
		"-e", "MINIO_ROOT_USER=" + rootUser, "-e", "MINIO_ROOT_PASSWORD=" + rootPassword,
		"-p", "127.0.0.1::9000", image}
	// "server" works as the arguments of the Chainguard image (entrypoint minio) and of
	// images whose command is `minio`; for the latter the command is replaced entirely.
	if strings.Contains(image, "pgsty") {
		args = append(args, "minio")
	}
	args = append(args, "server", "/tmp/data", "--address", ":9000")
	out, err := exec.Command("docker", args...).CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("docker run: %v: %s", err, out)
	}
	stop = func() { _ = exec.Command("docker", "rm", "-f", name).Run() }
	out, err = exec.Command("docker", "port", name, "9000/tcp").Output()
	if err != nil {
		stop()
		return nil, fmt.Errorf("docker port: %v", err)
	}
	addr := strings.TrimSpace(strings.SplitN(string(out), "\n", 2)[0]) // 127.0.0.1:49153
	server.endpoint = "http://" + addr
	server.accessKey, server.secretKey = rootUser, rootPassword
	deadline := time.Now().Add(60 * time.Second)
	for time.Now().Before(deadline) {
		if resp, err := http.Get(server.endpoint + "/minio/health/ready"); err == nil {
			resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return stop, nil
			}
		}
		time.Sleep(300 * time.Millisecond)
	}
	logs, _ := exec.Command("docker", "logs", name).CombinedOutput()
	stop()
	return nil, fmt.Errorf("MinIO did not become ready: %s", logs)
}

func config(bucket string) domain.ObjectStoreConfig {
	return domain.ObjectStoreConfig{
		Driver: domain.StorageS3, Endpoint: server.endpoint, Region: server.region, Bucket: bucket,
		Addressing: server.addressing, AccessKeyID: server.accessKey, SecretAccessKey: server.secretKey,
	}
}

// admin is a raw client for creating and removing the buckets the tests use.
func admin() *s3.Client {
	return s3.New(s3.Options{
		Region: server.region, BaseEndpoint: aws.String(server.endpoint), UsePathStyle: server.addressing != domain.AddressingVirtualHost,
		Credentials:                credentials.NewStaticCredentialsProvider(server.accessKey, server.secretKey, ""),
		RequestChecksumCalculation: aws.RequestChecksumCalculationWhenRequired,
		ResponseChecksumValidation: aws.ResponseChecksumValidationWhenRequired,
	})
}

func newBucket(t *testing.T) string {
	t.Helper()
	b := make([]byte, 6)
	_, _ = rand.Read(b)
	name := "xp-contract-" + hex.EncodeToString(b)
	ctx := context.Background()
	if _, err := admin().CreateBucket(ctx, &s3.CreateBucketInput{Bucket: &name}); err != nil {
		t.Fatalf("create bucket: %v", err)
	}
	t.Cleanup(func() {
		c := admin()
		for {
			page, err := c.ListObjects(ctx, &s3.ListObjectsInput{Bucket: &name})
			if err != nil || len(page.Contents) == 0 {
				break
			}
			for _, o := range page.Contents {
				_, _ = c.DeleteObject(ctx, &s3.DeleteObjectInput{Bucket: &name, Key: o.Key})
			}
		}
		_, _ = c.DeleteBucket(ctx, &s3.DeleteBucketInput{Bucket: &name})
	})
	return name
}

func TestS3SatisfiesTheContract(t *testing.T) {
	storetest.Run(t, func(t *testing.T) domain.ObjectStore {
		s, err := objectstore.NewS3(config(newBucket(t)))
		if err != nil {
			t.Fatal(err)
		}
		return s
	})
}

func TestS3ReportsWhatIsWrongWithTheConnection(t *testing.T) {
	ctx := context.Background()
	cases := []struct {
		name string
		cfg  func(c domain.ObjectStoreConfig) domain.ObjectStoreConfig
		want error
	}{
		{"unknown bucket", func(c domain.ObjectStoreConfig) domain.ObjectStoreConfig { c.Bucket = "xp-no-such-bucket"; return c }, domain.ErrStorageBucketNotFound},
		{"wrong secret key", func(c domain.ObjectStoreConfig) domain.ObjectStoreConfig { c.SecretAccessKey = "wrong"; return c }, domain.ErrStorageAccessDenied},
		{"wrong access key", func(c domain.ObjectStoreConfig) domain.ObjectStoreConfig { c.AccessKeyID = "nobody"; return c }, domain.ErrStorageAccessDenied},
		{"nothing listens", func(c domain.ObjectStoreConfig) domain.ObjectStoreConfig { c.Endpoint = "http://127.0.0.1:1"; return c }, domain.ErrStorageUnreachable},
	}
	bucket := newBucket(t)
	for _, tc := range cases {
		s, err := objectstore.NewS3(tc.cfg(config(bucket)))
		if err != nil {
			t.Fatalf("%s: %v", tc.name, err)
		}
		if err := s.TestConnection(ctx); !errors.Is(err, tc.want) {
			t.Errorf("%s: TestConnection err = %v, want %v", tc.name, err, tc.want)
		}
	}
}
