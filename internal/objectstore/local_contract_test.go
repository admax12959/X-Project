//go:build objectstorecontract

// Contract test of the local-disk adapter: the shared suite against a temporary directory.
// Not part of the default test set; run with make test-storage-contract.
package objectstore_test

import (
	"testing"

	"github.com/admax12959/X-Project/internal/domain"
	"github.com/admax12959/X-Project/internal/objectstore"
	"github.com/admax12959/X-Project/internal/objectstore/storetest"
)

func TestLocalDiskSatisfiesTheContract(t *testing.T) {
	storetest.Run(t, func(t *testing.T) domain.ObjectStore {
		s, err := objectstore.NewLocal(t.TempDir())
		if err != nil {
			t.Fatal(err)
		}
		return s
	})
}
