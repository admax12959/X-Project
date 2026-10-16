package domain

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Avatars (#32, ADR-0007). A user has at most one current avatar object in the object store,
// "avatars/<user id>/<content hash>-<random>.jpg"; the users row says which. Browsers never see
// the store: GET /api/avatars/{id} checks the session and streams the object.

// AvatarSource says who put the current avatar there.
type AvatarSource string

const (
	AvatarUpload AvatarSource = "upload" // the user uploaded it; wins over everything else
	AvatarLDAP   AvatarSource = "ldap"   // the directory photo, put through the same pipeline
)

const (
	avatarPrefix = "avatars/"

	// A user may start this many uploads (successful or not) per window.
	avatarUploadsPerWindow = 10
	avatarUploadWindow     = 10 * time.Minute

	// AvatarOrphanMinAge is what the periodic cleanup passes to SweepAvatarOrphans: an object that
	// no user points to must be at least this old, so an upload that has written its object but
	// not yet saved the reference is never taken for an orphan.
	AvatarOrphanMinAge = time.Hour
)

// avatarKey is the key of one upload: the content hash and a random suffix. The key is not
// content-addressed, on purpose: with A, B and A again in quick succession a content key would
// bring the user back to an object that the slower request replacing A is about to delete.
// A key, once replaced, can never become current again.
func avatarKey(id UserID, hash string) (string, error) {
	suffix := make([]byte, 8)
	if _, err := rand.Read(suffix); err != nil {
		return "", err
	}
	return avatarPrefix + strconv.FormatInt(int64(id), 10) + "/" + hash + "-" + hex.EncodeToString(suffix) + ".jpg", nil
}

// AvatarRepository is the part of the persistence port the avatars need.
type AvatarRepository interface {
	// SetUserAvatar replaces the user's current avatar (an empty key clears it) and returns the
	// object key it replaced. Call inside InTx: it locks the user's row. With replaceUpload
	// false it leaves an uploaded avatar alone (applied=false); the same for an unknown user.
	SetUserAvatar(ctx context.Context, id UserID, key, hash string, source AvatarSource, replaceUpload bool) (previous string, applied bool, err error)
	// AvatarKeysInUse returns the object keys users currently point to.
	AvatarKeysInUse(ctx context.Context) (map[string]bool, error)
}

// ---------------------------------------------------------------- upload rate limit

// uploadLimiter allows each user a number of uploads per sliding window. It lives in memory,
// per process: it only has to stop one person from hammering the image pipeline.
type uploadLimiter struct {
	mu    sync.Mutex
	times map[UserID][]time.Time
}

func (l *uploadLimiter) allow(id UserID, now time.Time) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.times == nil {
		l.times = map[UserID][]time.Time{}
	}
	recent := l.times[id][:0]
	for _, t := range l.times[id] {
		if now.Sub(t) < avatarUploadWindow {
			recent = append(recent, t)
		}
	}
	if len(recent) >= avatarUploadsPerWindow {
		l.times[id] = recent
		return false
	}
	l.times[id] = append(recent, now)
	return true
}

// ---------------------------------------------------------------- own avatar

// SetAvatar replaces the actor's avatar with the uploaded file (data is at most MaxAvatarBytes+1
// bytes: the HTTP layer reads one byte more than allowed so that a bigger file is noticed).
// Errors: 429 rateLimited, 422 avatar avatarTooLarge / avatarUnsupportedType, 503 storageUnavailable.
func (s *Service) SetAvatar(ctx context.Context, actor Actor, data []byte) (UserWithIdentities, error) {
	if !s.avatarLimits.allow(actor.User.ID, s.now()) {
		return UserWithIdentities{}, newKind(KindTooManyRequests, CodeRateLimited)
	}
	jpg, hash, err := processAvatar(data)
	if err != nil {
		return UserWithIdentities{}, err
	}
	store, err := s.ObjectStore(ctx)
	if err != nil {
		return UserWithIdentities{}, err
	}
	if _, err := s.storeAvatar(ctx, store, actor.User.ID, jpg, hash, AvatarUpload, true); err != nil {
		return UserWithIdentities{}, err
	}
	return s.GetMe(ctx, actor)
}

// RemoveAvatar removes the actor's uploaded avatar; the directory photo, if there is one, shows
// again. An avatar that came from the directory is not the user's to remove: nothing happens.
func (s *Service) RemoveAvatar(ctx context.Context, actor Actor) (UserWithIdentities, error) {
	var previous string
	err := s.repo.InTx(ctx, func(tx Repository) error {
		u, found, err := tx.GetUser(ctx, actor.User.ID)
		if err != nil {
			return err
		}
		if !found {
			return newKind(KindNotFound, CodeNotFound)
		}
		if u.AvatarSource != AvatarUpload {
			return nil
		}
		previous, _, err = tx.SetUserAvatar(ctx, u.ID, "", "", "", true)
		return err
	})
	if err != nil {
		return UserWithIdentities{}, err
	}
	if previous != "" {
		s.deleteAvatarObject(ctx, previous)
		s.restoreDirectoryPhoto(ctx, actor.User.ID)
	}
	return s.GetMe(ctx, actor)
}

// storeAvatar writes the object, points the user at it and deletes the object it replaces: new
// first, old second, so a failure in between leaves a harmless orphan and never a user with a
// dangling avatar. applied=false when replaceUpload is false and the user has an upload.
func (s *Service) storeAvatar(ctx context.Context, store ObjectStore, id UserID, jpg []byte, hash string, source AvatarSource, replaceUpload bool) (applied bool, err error) {
	key, err := avatarKey(id, hash)
	if err != nil {
		return false, err
	}
	if err := store.Put(ctx, key, jpg, "image/jpeg"); err != nil {
		return false, storageFailure("put avatar", err)
	}
	var previous string
	err = s.repo.InTx(ctx, func(tx Repository) error {
		var err error
		previous, applied, err = tx.SetUserAvatar(ctx, id, key, hash, source, replaceUpload)
		return err
	})
	if err != nil {
		return false, err // the new object stays behind; the periodic cleanup removes it
	}
	if previous != "" { // keys are unique per upload: the previous object is never the new one
		s.deleteAvatarObject(ctx, previous)
	}
	return applied, nil
}

func (s *Service) deleteAvatarObject(ctx context.Context, key string) {
	store, err := s.ObjectStore(ctx)
	if err == nil {
		err = store.Delete(ctx, key)
	}
	if err != nil {
		slog.Warn("avatar: could not delete an old object; the periodic cleanup will retry", "key", key, "err", err)
	}
}

func storageFailure(what string, err error) error {
	slog.Error("avatar: object storage failed", "operation", what, "err", err)
	return storageUnavailable()
}

// ---------------------------------------------------------------- download

// AvatarHash returns the content hash of the user's current avatar (the version in its URL).
// 404 notFound when the user has none.
func (s *Service) AvatarHash(ctx context.Context, id UserID) (string, error) {
	u, found, err := s.repo.GetUser(ctx, id)
	if err != nil {
		return "", err
	}
	if !found || u.AvatarKey == "" {
		return "", newKind(KindNotFound, CodeNotFound)
	}
	return u.AvatarHash, nil
}

// ReadAvatar returns the JPEG of the user's current avatar and its content hash. 404 notFound
// when the user has none; 503 storageUnavailable when the object store fails.
func (s *Service) ReadAvatar(ctx context.Context, id UserID) (jpg []byte, hash string, err error) {
	u, found, err := s.repo.GetUser(ctx, id)
	if err != nil {
		return nil, "", err
	}
	if !found || u.AvatarKey == "" {
		return nil, "", newKind(KindNotFound, CodeNotFound)
	}
	store, err := s.ObjectStore(ctx)
	if err != nil {
		return nil, "", err
	}
	body, err := store.Open(ctx, u.AvatarKey)
	if errors.Is(err, ErrObjectNotFound) {
		slog.Warn("avatar: the object of a user's avatar is missing", "user", id, "key", u.AvatarKey)
		return nil, "", newKind(KindNotFound, CodeNotFound)
	}
	if err != nil {
		return nil, "", storageFailure("open avatar", err)
	}
	defer body.Close()
	jpg, err = io.ReadAll(io.LimitReader(body, MaxAvatarBytes))
	if err != nil {
		return nil, "", storageFailure("read avatar", err)
	}
	return jpg, u.AvatarHash, nil
}

// ---------------------------------------------------------------- directory photo

// syncDirectoryPhoto makes the user's avatar the directory photo (display priority: an uploaded
// avatar > the directory photo > initials). It runs after a successful LDAP login and must never
// make the login fail: a photo that is not a usable picture, or a storage that is down, is
// logged and ignored. An uploaded avatar is never touched; an unchanged photo is not rewritten;
// a directory that no longer has a photo takes its old photo away.
func (s *Service) syncDirectoryPhoto(ctx context.Context, u User, photo []byte) {
	if u.AvatarSource == AvatarUpload {
		return
	}
	if len(photo) == 0 {
		if u.AvatarSource != AvatarLDAP {
			return
		}
		var previous string
		err := s.repo.InTx(ctx, func(tx Repository) error {
			var err error
			previous, _, err = tx.SetUserAvatar(ctx, u.ID, "", "", "", false)
			return err
		})
		if err != nil {
			slog.Warn("avatar: could not remove the directory photo", "user", u.ID, "err", err)
			return
		}
		if previous != "" {
			s.deleteAvatarObject(ctx, previous)
		}
		return
	}
	jpg, hash, err := processAvatar(photo)
	if err != nil {
		slog.Warn("avatar: the directory photo is not a usable picture", "user", u.ID, "err", describeAvatarError(err))
		return
	}
	if u.AvatarSource == AvatarLDAP && u.AvatarHash == hash {
		return
	}
	store, err := s.ObjectStore(ctx)
	if err == nil {
		_, err = s.storeAvatar(ctx, store, u.ID, jpg, hash, AvatarLDAP, false)
	}
	if err != nil {
		slog.Warn("avatar: could not store the directory photo", "user", u.ID, "err", describeAvatarError(err))
	}
}

func describeAvatarError(err error) string {
	if de, ok := AsError(err); ok {
		var codes []string
		for _, f := range de.Errors {
			codes = append(codes, f.Code)
		}
		return strings.Join(codes, ",")
	}
	return fmt.Sprint(err)
}

// restoreDirectoryPhoto fetches the directory photo of a user who just removed their upload.
// Best effort: without LDAP, without an LDAP identity or with the directory down, the user
// simply shows initials until the next LDAP login.
func (s *Service) restoreDirectoryPhoto(ctx context.Context, id UserID) {
	ident, found, err := s.repo.FindIdentityByUser(ctx, id, IdentityLDAP)
	if err != nil || !found {
		return
	}
	dir, err := s.currentDirectory(ctx)
	if err != nil || dir == nil {
		return
	}
	res := dir.Lookup(ctx, ident.LoginName)
	if res.Outcome != DirectoryOK {
		return
	}
	u, found, err := s.repo.GetUser(ctx, id)
	if err != nil || !found {
		return
	}
	s.syncDirectoryPhoto(ctx, u, res.Entry.Photo)
}

// ---------------------------------------------------------------- cleanup

// RunAvatarSweeper runs SweepAvatarOrphans every interval until ctx ends. The first pass comes one
// interval after the start. A storage that is down or not configured yet is logged, not fatal.
func (s *Service) RunAvatarSweeper(ctx context.Context, interval time.Duration) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-time.After(interval):
		}
		n, err := s.SweepAvatarOrphans(ctx, AvatarOrphanMinAge)
		if err != nil && ctx.Err() == nil {
			slog.Error("avatar: sweeping orphan objects", "err", err)
		} else if n > 0 {
			slog.Info("avatar: deleted orphan objects", "count", n)
		}
	}
}

// SweepAvatarOrphans deletes the objects under avatars/ that no user points to and that are at
// least minAge old, and returns how many it deleted. Uploads and removals delete the object they
// replace, so orphans only come from failures in between (a storage that was down, a crash).
func (s *Service) SweepAvatarOrphans(ctx context.Context, minAge time.Duration) (int, error) {
	store, err := s.ObjectStore(ctx)
	if err != nil {
		return 0, err
	}
	objects, err := store.List(ctx, avatarPrefix) // listed before the references are read: an object written later is never judged by an older list
	if err != nil {
		return 0, storageFailure("list avatars", err)
	}
	inUse, err := s.repo.AvatarKeysInUse(ctx)
	if err != nil {
		return 0, err
	}
	now := s.now()
	deleted := 0
	for _, o := range objects {
		if inUse[o.Key] || now.Sub(o.ModTime) < minAge {
			continue
		}
		if err := store.Delete(ctx, o.Key); err != nil {
			slog.Warn("avatar: could not delete an orphan object", "key", o.Key, "err", err)
			continue
		}
		deleted++
	}
	return deleted, nil
}
