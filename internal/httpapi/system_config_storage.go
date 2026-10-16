package httpapi

import "github.com/admax12959/X-Project/internal/domain"

// Object storage configuration (#24, ADR-0007). Same rules and routes as the other system
// configuration sections (system_config.go): administrators only, secrets write-only.

// storageConfigJSON is the response: the form without the key. accessKeyIdSet and
// secretAccessKeySet say whether the parts of the key are stored; localDir is where the local
// driver keeps objects.
type storageConfigJSON struct {
	domain.StorageSettings
	AccessKeyIDSet     bool   `json:"accessKeyIdSet"`
	SecretAccessKeySet bool   `json:"secretAccessKeySet"`
	LocalDir           string `json:"localDir"`
	configStatusJSON
}

func toStorageConfigJSON(v domain.StorageConfigView) any {
	return storageConfigJSON{
		StorageSettings: v.StorageSettings, AccessKeyIDSet: v.AccessKeyIDSet, SecretAccessKeySet: v.SecretAccessKeySet,
		LocalDir:         v.LocalDir,
		configStatusJSON: newConfigStatus(v.SecretsUnreadable, v.Configured, v.Configured, v.UpdatedAt),
	}
}
