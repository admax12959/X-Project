package api

import (
	"bytes"
	"errors"
	"image/color"
	"net/http"
	"strings"
	"testing"

	"github.com/admax12959/X-Project/internal/apitest"
	"github.com/admax12959/X-Project/internal/domain"
)

// ---------------------------------------------------------------- what is accepted

func TestAvatarAcceptsJPEGPNGAndWebPAndAlwaysStoresASquareJPEG(t *testing.T) {
	t.Parallel()
	env := apitest.New(t)
	for name, file := range map[string][]byte{
		"jpeg": jpegBytes(t, twoBand(500, 300)),
		"png":  pngBytes(t, twoBand(300, 500)),
		"webp": webpFixture(t),
	} {
		c := plainUserClient(t, env, "u"+name)
		url := uploadedAvatarURL(t, c, file)
		img, res := fetchAvatar(t, c, url)
		if b := img.Bounds(); b.Dx() != 256 || b.Dy() != 256 {
			t.Errorf("%s: avatar is %dx%d, want 256x256", name, b.Dx(), b.Dy())
		}
		if ct := res.Header.Get("Content-Type"); ct != "image/jpeg" {
			t.Errorf("%s: Content-Type = %q, want image/jpeg", name, ct)
		}
	}
}

func TestAvatarSmallPicturesAreScaledUpToTheSameSquare(t *testing.T) {
	t.Parallel()
	env := apitest.New(t)
	c := plainUserClient(t, env, "jane")
	img, _ := fetchAvatar(t, c, uploadedAvatarURL(t, c, pngBytes(t, twoBand(40, 30))))
	if b := img.Bounds(); b.Dx() != 256 || b.Dy() != 256 {
		t.Fatalf("avatar is %dx%d, want 256x256", b.Dx(), b.Dy())
	}
}

// ---------------------------------------------------------------- what is refused

func TestAvatarRefusesFilesThatAreNotJPEGPNGOrWebP(t *testing.T) {
	t.Parallel()
	env := apitest.New(t)
	c := plainUserClient(t, env, "jane")
	png := pngBytes(t, twoBand(64, 64))

	cases := map[string][]byte{
		"svg":                        []byte(`<svg xmlns="http://www.w3.org/2000/svg" width="64" height="64"><script>alert(1)</script></svg>`),
		"svg with xml header":        []byte(`<?xml version="1.0"?><svg xmlns="http://www.w3.org/2000/svg"/>`),
		"html":                       []byte(`<html><script>alert(1)</script></html>`),
		"plain text":                 []byte("not a picture at all"),
		"gif":                        []byte("GIF89a\x01\x00\x01\x00\x00\x00\x00;"),
		"empty":                      {},
		"truncated png":              png[:len(png)/2],
		"png header only":            png[:16],
		"svg behind a png signature": append(append([]byte{}, png[:8]...), []byte(`<svg xmlns="http://www.w3.org/2000/svg"/>`)...),
	}
	for _, file := range cases {
		// The declared content type must not matter: say it is a PNG every time.
		res := c.Raw(http.MethodPut, "/api/me/avatar", file, "image/png")
		res.ExpectError(http.StatusUnprocessableEntity, "avatar", domain.CodeAvatarUnsupportedType)
	}
	if n := avatarObjectCount(t, env); n != 0 {
		t.Fatalf("%d objects stored after only refused uploads", n)
	}
	var me avatarUserBody
	c.Get("/api/me").ExpectStatus(http.StatusOK).JSON(&me)
	if me.AvatarURL != nil {
		t.Fatalf("a refused upload left avatarUrl %q", *me.AvatarURL)
	}
}

func TestAvatarRefusesMoreThanTwoMegabytes(t *testing.T) {
	t.Parallel()
	env := apitest.New(t)
	c := plainUserClient(t, env, "jane")

	big := append(pngBytes(t, twoBand(64, 64)), bytes.Repeat([]byte{0}, domain.MaxAvatarBytes)...) // a real PNG plus padding
	e := uploadAvatar(c, big).ExpectError(http.StatusUnprocessableEntity, "avatar", domain.CodeAvatarTooLarge)
	if e.Params["maxBytes"] != float64(2<<20) {
		t.Fatalf("params = %v, want maxBytes 2097152", e.Params)
	}
	if n := avatarObjectCount(t, env); n != 0 {
		t.Fatalf("%d objects stored after a refused upload", n)
	}
}

func TestAvatarChecksThePixelCountBeforeDecoding(t *testing.T) {
	t.Parallel()
	env := apitest.New(t)
	c := plainUserClient(t, env, "jane")

	// A small file whose header announces 60000x60000 pixels: it must be refused without decoding.
	bomb := jpegClaimingSize(t, jpegBytes(t, twoBand(32, 32)), 60000, 60000)
	uploadAvatar(c, bomb).ExpectError(http.StatusUnprocessableEntity, "avatar", domain.CodeAvatarTooLarge)

	// The same for a PNG header (IHDR width/height are bytes 16..24; the CRC is not looked at before the size check).
	png := pngBytes(t, twoBand(32, 32))
	png[16], png[17], png[18], png[19] = 0, 0, 0xEA, 0x60
	png[20], png[21], png[22], png[23] = 0, 0, 0xEA, 0x60
	if res := uploadAvatar(c, png); res.Status != http.StatusUnprocessableEntity {
		t.Fatalf("PNG bomb: status %d, want 422: %s", res.Status, res.Body)
	}
	if n := avatarObjectCount(t, env); n != 0 {
		t.Fatalf("%d objects stored after refused uploads", n)
	}
}

// ---------------------------------------------------------------- orientation and metadata

func TestAvatarIsTurnedUprightByItsEXIFOrientationAndLosesItsMetadata(t *testing.T) {
	t.Parallel()
	env := apitest.New(t)
	c := plainUserClient(t, env, "jane")

	// Stored as 600x300 with red on top; orientation 6 says "rotate 90 degrees clockwise to
	// view it", so the red top ends up on the right and the blue bottom on the left.
	file := withJPEGMetadata(jpegBytes(t, twoBand(600, 300)), 6)
	if !bytes.Contains(file, []byte("SECRET-COMMENT")) {
		t.Fatal("fixture lost its metadata")
	}
	img, res := fetchAvatar(t, c, uploadedAvatarURL(t, c, file))
	if !nearColor(img, 20, 128, colBlue) || !nearColor(img, 235, 128, colRed) {
		t.Fatalf("orientation 6 not applied: left=%v right=%v", img.At(20, 128), img.At(235, 128))
	}
	body := res.Body
	for _, leak := range []string{"SECRET-COMMENT", "Exif", "GPS"} {
		if bytes.Contains(body, []byte(leak)) {
			t.Fatalf("the stored avatar still contains %q", leak)
		}
	}

	// Without an orientation tag the picture stays as it is: red on top.
	img2, _ := fetchAvatar(t, c, uploadedAvatarURL(t, c, jpegBytes(t, twoBand(600, 300))))
	if !nearColor(img2, 128, 20, colRed) || !nearColor(img2, 128, 235, colBlue) {
		t.Fatalf("an untagged picture was rotated: top=%v bottom=%v", img2.At(128, 20), img2.At(128, 235))
	}
}

func TestAvatarIsCroppedFromTheCentre(t *testing.T) {
	t.Parallel()
	env := apitest.New(t)
	c := plainUserClient(t, env, "jane")
	// 400x100: a red band in the middle 100px, blue on both sides. The centre square is all red.
	src := solid(400, 100, color.NRGBA{B: 220, A: 255})
	for y := 0; y < 100; y++ {
		for x := 150; x < 250; x++ {
			src.SetNRGBA(x, y, color.NRGBA{R: 220, A: 255})
		}
	}
	img, _ := fetchAvatar(t, c, uploadedAvatarURL(t, c, pngBytes(t, src)))
	for _, p := range [][2]int{{30, 30}, {128, 128}, {225, 225}} {
		if !nearColor(img, p[0], p[1], colRed) {
			t.Fatalf("pixel %v = %v, want red (the centre square)", p, img.At(p[0], p[1]))
		}
	}
}

// ---------------------------------------------------------------- replace and remove

func TestReplacingAnAvatarKeepsOnlyTheNewObject(t *testing.T) {
	t.Parallel()
	env := apitest.New(t)
	jane := plainUserClient(t, env, "jane")
	bob := plainUserClient(t, env, "bob")

	first := uploadedAvatarURL(t, jane, pngBytes(t, twoBand(300, 300)))
	uploadedAvatarURL(t, bob, pngBytes(t, solid(300, 300, color.NRGBA{G: 200, A: 255})))
	if n := avatarObjectCount(t, env); n != 2 {
		t.Fatalf("%d objects for two users, want 2", n)
	}

	second := uploadedAvatarURL(t, jane, pngBytes(t, solid(300, 300, color.NRGBA{R: 10, G: 10, B: 200, A: 255})))
	if second == first {
		t.Fatalf("a new picture kept the old URL %q: caches would show the old one", first)
	}
	if n := avatarObjectCount(t, env); n != 2 {
		t.Fatalf("%d objects after replacing, want 2 (one per user)", n)
	}
	janeID := strings.TrimSuffix(strings.TrimPrefix(second, "/api/avatars/"), second[strings.Index(second, "?"):])
	if objs, _ := env.Objects.List(t.Context(), "avatars/"+janeID+"/"); len(objs) != 1 {
		t.Fatalf("jane has %d objects, want exactly 1", len(objs))
	}
	// The new picture is what is served, immediately, to everyone.
	img, _ := fetchAvatar(t, bob, second)
	if !nearColor(img, 128, 128, color.RGBA{R: 10, G: 10, B: 200, A: 255}) {
		t.Fatalf("the served avatar is %v, want the new picture", img.At(128, 128))
	}
}

func TestAnAvatarThatCannotBeStoredLeavesTheOldOneInPlace(t *testing.T) {
	t.Parallel()
	env := apitest.New(t)
	jane := plainUserClient(t, env, "jane")
	first := uploadedAvatarURL(t, jane, pngBytes(t, twoBand(300, 300)))

	env.Objects.SetFailure(errors.New("storage is down"))
	uploadAvatar(jane, pngBytes(t, twoBand(200, 300))).ExpectStatus(http.StatusServiceUnavailable)
	env.Objects.SetFailure(nil)

	var me avatarUserBody
	jane.Get("/api/me").ExpectStatus(http.StatusOK).JSON(&me)
	if me.AvatarURL == nil || *me.AvatarURL != first {
		t.Fatalf("avatarUrl = %v after a failed upload, want the old %q", me.AvatarURL, first)
	}
	fetchAvatar(t, jane, first)
}

func TestRemovingAnAvatarDeletesTheObjectAndFallsBackToInitials(t *testing.T) {
	t.Parallel()
	env := apitest.New(t)
	jane := plainUserClient(t, env, "jane")
	url := uploadedAvatarURL(t, jane, pngBytes(t, twoBand(300, 300)))

	var me avatarUserBody
	jane.Delete("/api/me/avatar").ExpectStatus(http.StatusOK).JSON(&me)
	if me.AvatarURL != nil {
		t.Fatalf("avatarUrl = %q after removing, want null", *me.AvatarURL)
	}
	if n := avatarObjectCount(t, env); n != 0 {
		t.Fatalf("%d objects left after removing", n)
	}
	jane.Get(url).ExpectStatus(http.StatusNotFound)

	// Removing when there is nothing to remove is not an error.
	jane.Delete("/api/me/avatar").ExpectStatus(http.StatusOK)
	env.Client().Delete("/api/me/avatar").ExpectStatus(http.StatusUnauthorized)
}

// ---------------------------------------------------------------- rate limit

func TestAvatarUploadsAreRateLimitedPerUser(t *testing.T) {
	t.Parallel()
	env := apitest.New(t)
	jane := plainUserClient(t, env, "jane")
	bob := plainUserClient(t, env, "bob")
	png := pngBytes(t, twoBand(64, 64))

	for i := 0; i < 10; i++ {
		uploadAvatar(jane, png).ExpectStatus(http.StatusOK)
	}
	uploadAvatar(jane, png).ExpectError(http.StatusTooManyRequests, "", domain.CodeRateLimited)
	// Refused files count as attempts too: the limit protects the image pipeline.
	uploadAvatar(jane, []byte("junk")).ExpectError(http.StatusTooManyRequests, "", domain.CodeRateLimited)
	// Somebody else is not affected.
	uploadAvatar(bob, png).ExpectStatus(http.StatusOK)
}

// ---------------------------------------------------------------- orphan cleanup

func TestTheCleanupDeletesOrphanObjectsAndKeepsCurrentAvatars(t *testing.T) {
	t.Parallel()
	env := apitest.New(t)
	jane := plainUserClient(t, env, "jane")
	url := uploadedAvatarURL(t, jane, pngBytes(t, twoBand(300, 300)))

	// Leftovers of failures: an old object of jane, an object of a user that is gone.
	for _, key := range []string{"avatars/1/deadbeefdeadbeef.jpg", "avatars/987654/cafebabecafebabe.jpg"} {
		if err := env.Objects.Put(t.Context(), key, []byte("x"), "image/jpeg"); err != nil {
			t.Fatal(err)
		}
	}
	// Something that is not an avatar is none of the cleanup's business.
	if err := env.Objects.Put(t.Context(), "other/keep.txt", []byte("x"), "text/plain"); err != nil {
		t.Fatal(err)
	}
	if n := avatarObjectCount(t, env); n != 3 {
		t.Fatalf("%d avatar objects before the cleanup, want 3", n)
	}

	deleted, err := env.SweepAvatarOrphans(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if deleted != 2 || avatarObjectCount(t, env) != 1 {
		t.Fatalf("deleted %d, %d avatar objects left; want 2 deleted, 1 left", deleted, avatarObjectCount(t, env))
	}
	fetchAvatar(t, jane, url) // the current avatar survived
	if _, err := env.Objects.Open(t.Context(), "other/keep.txt"); err != nil {
		t.Fatalf("the cleanup touched a non-avatar object: %v", err)
	}
}

// ---------------------------------------------------------------- display priority (LDAP photo)

func ldapEntryWithPhoto(photo []byte) domain.DirectoryEntry {
	return domain.DirectoryEntry{DisplayName: "Emma Larsen", Email: "emma.larsen@corp.example", Department: "Payments Platform", Photo: photo}
}

func avatarOf(t *testing.T, c *apitest.Client) *string {
	t.Helper()
	var me avatarUserBody
	c.Get("/api/me").ExpectStatus(http.StatusOK).JSON(&me)
	return me.AvatarURL
}

func TestTheDirectoryPhotoIsProcessedLikeAnUploadAndShownWhenThereIsNoUpload(t *testing.T) {
	t.Parallel()
	env := apitest.New(t)
	env.Directory.AddAccount("elarsen", "ldap-pw", ldapEntryWithPhoto(withJPEGMetadata(jpegBytes(t, twoBand(640, 480)), 1)))

	emma := env.Client()
	loginLDAP(emma, "elarsen", "ldap-pw").ExpectStatus(http.StatusOK)
	url := avatarOf(t, emma)
	if url == nil || !avatarURLPattern.MatchString(*url) {
		t.Fatalf("avatarUrl = %v after an LDAP login with a photo", url)
	}
	img, res := fetchAvatar(t, emma, *url)
	if b := img.Bounds(); b.Dx() != 256 || b.Dy() != 256 {
		t.Fatalf("directory avatar is %dx%d, want 256x256", b.Dx(), b.Dy())
	}
	if bytes.Contains(res.Body, []byte("SECRET-COMMENT")) {
		t.Fatal("the directory photo kept its metadata")
	}

	// Logging in again with the same photo changes nothing.
	emma2 := env.Client()
	loginLDAP(emma2, "elarsen", "ldap-pw").ExpectStatus(http.StatusOK)
	if again := avatarOf(t, emma2); again == nil || *again != *url {
		t.Fatalf("avatarUrl = %v after a second login with the same photo, want %q", again, *url)
	}
	if n := avatarObjectCount(t, env); n != 1 {
		t.Fatalf("%d avatar objects, want 1", n)
	}
}

func TestAnUploadedAvatarBeatsTheDirectoryPhotoAndRemovingItBringsThePhotoBack(t *testing.T) {
	t.Parallel()
	env := apitest.New(t)
	env.Directory.AddAccount("elarsen", "ldap-pw", ldapEntryWithPhoto(jpegBytes(t, solid(300, 300, color.NRGBA{G: 200, A: 255}))))
	emma := env.Client()
	loginLDAP(emma, "elarsen", "ldap-pw").ExpectStatus(http.StatusOK)
	photoURL := *avatarOf(t, emma)

	uploaded := uploadedAvatarURL(t, emma, pngBytes(t, twoBand(300, 300)))
	if uploaded == photoURL {
		t.Fatal("the upload did not replace the directory photo")
	}

	// A later LDAP login (with a changed photo) must not take the upload away.
	env.Directory.AddAccount("elarsen", "ldap-pw", ldapEntryWithPhoto(jpegBytes(t, solid(300, 300, color.NRGBA{B: 200, A: 255}))))
	again := env.Client()
	loginLDAP(again, "elarsen", "ldap-pw").ExpectStatus(http.StatusOK)
	if got := avatarOf(t, again); got == nil || *got != uploaded {
		t.Fatalf("avatarUrl = %v after an LDAP login, want the upload %q", got, uploaded)
	}
	if n := avatarObjectCount(t, env); n != 1 {
		t.Fatalf("%d avatar objects, want 1", n)
	}

	// Removing the upload shows the directory photo again.
	var me avatarUserBody
	again.Delete("/api/me/avatar").ExpectStatus(http.StatusOK).JSON(&me)
	if me.AvatarURL == nil {
		t.Fatal("after removing the upload the directory photo did not come back")
	}
	img, _ := fetchAvatar(t, again, *me.AvatarURL)
	if !nearColor(img, 128, 128, color.RGBA{B: 200, A: 255}) {
		t.Fatalf("the restored avatar is %v, want the (new) directory photo", img.At(128, 128))
	}
}

func TestADirectoryPhotoThatIsNotAPictureNeverBreaksTheLogin(t *testing.T) {
	t.Parallel()
	env := apitest.New(t)
	env.Directory.AddAccount("elarsen", "ldap-pw", ldapEntryWithPhoto([]byte("<svg xmlns='http://www.w3.org/2000/svg'/>")))
	emma := env.Client()
	loginLDAP(emma, "elarsen", "ldap-pw").ExpectStatus(http.StatusOK)
	if got := avatarOf(t, emma); got != nil {
		t.Fatalf("avatarUrl = %q from a photo that is not a picture", *got)
	}

	// The same when the storage is down: the login works, there is just no avatar.
	env.Directory.AddAccount("elarsen", "ldap-pw", ldapEntryWithPhoto(jpegBytes(t, twoBand(300, 300))))
	env.Objects.SetFailure(errors.New("storage is down"))
	loginLDAP(env.Client(), "elarsen", "ldap-pw").ExpectStatus(http.StatusOK)
	env.Objects.SetFailure(nil)
}

func TestADirectoryThatDropsThePhotoTakesTheAvatarAway(t *testing.T) {
	t.Parallel()
	env := apitest.New(t)
	env.Directory.AddAccount("elarsen", "ldap-pw", ldapEntryWithPhoto(jpegBytes(t, twoBand(300, 300))))
	emma := env.Client()
	loginLDAP(emma, "elarsen", "ldap-pw").ExpectStatus(http.StatusOK)
	if avatarOf(t, emma) == nil {
		t.Fatal("no avatar from the directory photo")
	}

	env.Directory.AddAccount("elarsen", "ldap-pw", ldapEntryWithPhoto(nil))
	again := env.Client()
	loginLDAP(again, "elarsen", "ldap-pw").ExpectStatus(http.StatusOK)
	if got := avatarOf(t, again); got != nil {
		t.Fatalf("avatarUrl = %q after the directory dropped the photo", *got)
	}
	if n := avatarObjectCount(t, env); n != 0 {
		t.Fatalf("%d avatar objects left", n)
	}
}

// ---------------------------------------------------------------- where colleagues see it

func TestAvatarsShowInTheUserListTheOwnerPickerAndTheApplicationMembers(t *testing.T) {
	t.Parallel()
	f := newAppFixture(t)
	emma := f.env.Client()
	emma.LoginAs("elarsen", "initial-pw-1").ExpectStatus(http.StatusOK)
	url := uploadedAvatarURL(t, emma, pngBytes(t, twoBand(300, 300)))
	f.post(map[string]any{}).ExpectStatus(http.StatusCreated)
	bob := plainUserClient(t, f.env, "bob") // no avatar

	byName := func(items []map[string]any, username string) map[string]any {
		for _, it := range items {
			if it["username"] == username {
				return it
			}
		}
		t.Fatalf("%s not in %v", username, items)
		return nil
	}
	check := func(what string, item map[string]any) {
		t.Helper()
		if got, _ := item["avatarUrl"].(string); got != url {
			t.Errorf("%s: avatarUrl = %v, want %q", what, item["avatarUrl"], url)
		}
	}

	var users struct {
		Items []map[string]any `json:"items"`
	}
	f.admin.Get("/api/users").ExpectStatus(http.StatusOK).JSON(&users)
	check("user list", byName(users.Items, "elarsen"))
	if v, has := byName(users.Items, "bob")["avatarUrl"]; !has || v != nil {
		t.Errorf("user list: bob avatarUrl = %v, want null", v)
	}

	var pickers struct {
		Items []map[string]any `json:"items"`
	}
	bob.Get("/api/assignable-users").ExpectStatus(http.StatusOK).JSON(&pickers)
	check("owner picker", byName(pickers.Items, "elarsen"))

	var a struct {
		Owners []map[string]any `json:"owners"`
	}
	bob.Get("/api/applications/payments-core").ExpectStatus(http.StatusOK).JSON(&a)
	check("application owners", byName(a.Owners, "elarsen"))
}
