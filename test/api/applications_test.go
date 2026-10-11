package api

import (
	"net/http"
	"testing"

	"github.com/admax12959/X-Project/internal/apitest"
)

func TestLoggedInUserSeesAnEmptyApplicationList(t *testing.T) {
	t.Parallel()
	env := apitest.New(t)
	res := env.AdminClient().Get("/api/applications").ExpectStatus(http.StatusOK)
	if got := string(res.Body); got != "{\"items\":[]}\n" {
		t.Fatalf("body = %q", got)
	}
}
