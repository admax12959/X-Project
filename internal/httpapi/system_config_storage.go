package httpapi

import (
	"net/http"

	"github.com/admax12959/X-Project/internal/domain"
)

// Object storage configuration (#24, ADR-0007). Same rules as the other system configuration
// sections (system_config.go): administrators only, secrets write-only.

// storageConfigBody is the request of PUT and POST .../test: the whole form. accessKeyId and
// secretAccessKey empty or absent keep the stored values.
type storageConfigBody struct {
	Driver          string `json:"driver"`
	Endpoint        string `json:"endpoint"`
	Region          string `json:"region"`
	Bucket          string `json:"bucket"`
	Addressing      string `json:"addressing"`
	AccessKeyID     string `json:"accessKeyId"`
	SecretAccessKey string `json:"secretAccessKey"`
}

func (b storageConfigBody) input() domain.StorageConfigInput {
	return domain.StorageConfigInput{
		AccessKeyID: b.AccessKeyID, SecretAccessKey: b.SecretAccessKey,
		StorageSettings: domain.StorageSettings{
			Driver: domain.ObjectStoreDriver(b.Driver), Endpoint: b.Endpoint, Region: b.Region, Bucket: b.Bucket, Addressing: b.Addressing,
		},
	}
}

// storageConfigJSON is the response: the form without the key. accessKeyIdSet and
// secretAccessKeySet say whether the parts of the key are stored; secretsUnreadable that a
// stored secret cannot be decrypted with the current master key (enter it again); localDir is
// where the local driver keeps objects; configured that the section was ever saved.
type storageConfigJSON struct {
	Driver             string `json:"driver"`
	Endpoint           string `json:"endpoint"`
	Region             string `json:"region"`
	Bucket             string `json:"bucket"`
	Addressing         string `json:"addressing"`
	AccessKeyIDSet     bool   `json:"accessKeyIdSet"`
	SecretAccessKeySet bool   `json:"secretAccessKeySet"`
	SecretsUnreadable  bool   `json:"secretsUnreadable"`
	LocalDir           string `json:"localDir"`
	Configured         bool   `json:"configured"`
	UpdatedAt          string `json:"updatedAt,omitempty"`
}

func toStorageConfigJSON(v domain.StorageConfigView) storageConfigJSON {
	out := storageConfigJSON{
		Driver: string(v.Driver), Endpoint: v.Endpoint, Region: v.Region, Bucket: v.Bucket, Addressing: v.Addressing,
		AccessKeyIDSet: v.AccessKeyIDSet, SecretAccessKeySet: v.SecretAccessKeySet, SecretsUnreadable: v.SecretsUnreadable,
		LocalDir: v.LocalDir, Configured: v.Configured,
	}
	if v.Configured {
		out.UpdatedAt = formatTime(v.UpdatedAt)
	}
	return out
}

func (s *server) getStorageConfig(w http.ResponseWriter, r *http.Request) {
	v, err := s.svc.GetStorageConfig(r.Context(), actorOf(r))
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, toStorageConfigJSON(v))
}

func (s *server) putStorageConfig(w http.ResponseWriter, r *http.Request) {
	var body storageConfigBody
	if !decode(w, r, &body) {
		return
	}
	v, err := s.svc.UpdateStorageConfig(r.Context(), actorOf(r), body.input())
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, toStorageConfigJSON(v))
}

func (s *server) testStorageConfig(w http.ResponseWriter, r *http.Request) {
	var body storageConfigBody
	if !decode(w, r, &body) {
		return
	}
	res, err := s.svc.TestStorageConfig(r.Context(), actorOf(r), body.input())
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, connectionTestJSON{OK: res.OK, Code: res.Code})
}
