package fakes_test

import (
	"testing"

	"github.com/admax12959/X-Project/internal/domain"
	"github.com/admax12959/X-Project/internal/fakes"
	"github.com/admax12959/X-Project/internal/objectstore/storetest"
)

// The in-memory fake used by the API tests honours the same contract as the real adapters.
func TestMemoryObjectStoreSatisfiesTheContract(t *testing.T) {
	t.Parallel()
	storetest.Run(t, func(*testing.T) domain.ObjectStore { return fakes.NewObjectStore() })
}
