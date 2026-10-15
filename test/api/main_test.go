// Package api holds the HTTP API tests. Keep every API test in this one package:
// each test package process starts its own MySQL container.
package api

import (
	"testing"

	"github.com/admax12959/X-Project/internal/apitest"
)

func TestMain(m *testing.M) { apitest.Main(m) }
