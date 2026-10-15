package api

import (
	"bytes"
	"image"
	_ "image/jpeg"
	"net/http"
	"regexp"
	"strings"
	"testing"

	"github.com/admax12959/X-Project/internal/apitest"
)

// ---------------------------------------------------------------- helpers

// avatarUserBody is the part of the user shapes that carries the avatar.
type avatarUserBody struct {
	ID        string  `json:"id"`
	Username  string  `json:"username"`
	AvatarURL *string `json:"avatarUrl"`
}

var avatarURLPattern = regexp.MustCompile(`^/api/avatars/\d+\?v=[0-9a-f]{16}$`)

func uploadAvatar(c *apitest.Client, file []byte) *apitest.Response {
	return c.Raw(http.MethodPut, "/api/me/avatar", file, "application/octet-stream")
}

// uploadedAvatarURL uploads file as the client's avatar and returns the avatar URL the user view reports.
func uploadedAvatarURL(t *testing.T, c *apitest.Client, file []byte) string {
	t.Helper()
	var u avatarUserBody
	uploadAvatar(c, file).ExpectStatus(http.StatusOK).JSON(&u)
	if u.AvatarURL == nil {
		t.Fatal("avatarUrl is null after an upload")
	}
	return *u.AvatarURL
}

// fetchAvatar downloads an avatar URL and decodes it as an image.
func fetchAvatar(t *testing.T, c *apitest.Client, url string) (image.Image, *apitest.Response) {
	t.Helper()
	r := c.Get(url).ExpectStatus(http.StatusOK)
	img, _, err := image.Decode(bytes.NewReader(r.Body))
	if err != nil {
		t.Fatalf("the avatar is not a decodable image: %v", err)
	}
	return img, r
}

func avatarObjectCount(t *testing.T, env *apitest.Env) int {
	t.Helper()
	store, err := env.CurrentObjectStore(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	objs, err := store.List(t.Context(), "avatars/")
	if err != nil {
		t.Fatal(err)
	}
	return len(objs)
}

// ---------------------------------------------------------------- upload and download

func TestUserUploadsAnAvatarAndColleaguesSeeItThroughTheApp(t *testing.T) {
	t.Parallel()
	env := apitest.New(t)
	jane := plainUserClient(t, env, "jane")
	bob := plainUserClient(t, env, "bob")

	url := uploadedAvatarURL(t, jane, pngBytes(t, twoBand(600, 400)))
	if !avatarURLPattern.MatchString(url) {
		t.Fatalf("avatarUrl = %q, want /api/avatars/<user id>?v=<hash>", url)
	}

	// A colleague downloads it: a 256px square JPEG, cached for a long time, never sniffed.
	img, res := fetchAvatar(t, bob, url)
	if b := img.Bounds(); b.Dx() != 256 || b.Dy() != 256 {
		t.Fatalf("avatar is %dx%d, want 256x256", b.Dx(), b.Dy())
	}
	if ct := res.Header.Get("Content-Type"); ct != "image/jpeg" {
		t.Fatalf("Content-Type = %q, want image/jpeg", ct)
	}
	if v := res.Header.Get("X-Content-Type-Options"); v != "nosniff" {
		t.Fatalf("X-Content-Type-Options = %q, want nosniff", v)
	}
	cc := res.Header.Get("Cache-Control")
	if !strings.Contains(cc, "max-age=31536000") || !strings.Contains(cc, "immutable") || !strings.Contains(cc, "private") {
		t.Fatalf("Cache-Control = %q, want a private, immutable, one-year cache", cc)
	}

	// The user's own views report the same URL.
	var me avatarUserBody
	jane.Get("/api/me").ExpectStatus(http.StatusOK).JSON(&me)
	if me.AvatarURL == nil || *me.AvatarURL != url {
		t.Fatalf("GET /api/me avatarUrl = %v, want %q", me.AvatarURL, url)
	}
	var session struct {
		User avatarUserBody `json:"user"`
	}
	jane.Get("/api/session").ExpectStatus(http.StatusOK).JSON(&session)
	if session.User.AvatarURL == nil || *session.User.AvatarURL != url {
		t.Fatalf("GET /api/session avatarUrl = %v, want %q", session.User.AvatarURL, url)
	}
}

func TestAvatarsAreOnlyServedToLoggedInUsers(t *testing.T) {
	t.Parallel()
	env := apitest.New(t)
	jane := plainUserClient(t, env, "jane")
	url := uploadedAvatarURL(t, jane, pngBytes(t, twoBand(64, 64)))

	env.Client().Get(url).ExpectStatus(http.StatusUnauthorized)
	env.Client().Raw(http.MethodPut, "/api/me/avatar", pngBytes(t, twoBand(64, 64)), "image/png").ExpectStatus(http.StatusUnauthorized)

	// A user without an avatar, and a user that does not exist, have nothing to serve.
	bob := plainUserClient(t, env, "bob")
	var bobMe avatarUserBody
	bob.Get("/api/me").ExpectStatus(http.StatusOK).JSON(&bobMe)
	if bobMe.AvatarURL != nil {
		t.Fatalf("a user who never uploaded has avatarUrl %q", *bobMe.AvatarURL)
	}
	jane.Get("/api/avatars/" + bobMe.ID).ExpectStatus(http.StatusNotFound)
	jane.Get("/api/avatars/987654").ExpectStatus(http.StatusNotFound)
}
