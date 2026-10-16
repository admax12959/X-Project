package domain

import (
	"context"
	"net/url"
	"slices"
	"strings"
	"time"
)

// CodePlatform is a registered GitLab/GitHub instance (GLOSSARY: 代码平台).
// The token is never part of this struct; only its display tail is.
type CodePlatform struct {
	ID        int64
	Kind      PlatformKind
	BaseURL   string // normalized: http(s)://host[:port][/path], no trailing slash
	TokenTail string // last 4 characters of the token, for display; empty for short tokens
	CreatedAt time.Time
}

// CodePlatformRepository is the persistence port for code platforms.
type CodePlatformRepository interface {
	// CreateCodePlatform stores p with its already encrypted token (see tokenCipher) and returns the new ID.
	CreateCodePlatform(ctx context.Context, p CodePlatform, tokenEnc []byte) (int64, error)
	// ListCodePlatforms returns every platform, oldest first. It never returns tokens.
	ListCodePlatforms(ctx context.Context) ([]CodePlatform, error)
	// GetCodePlatform returns one platform with its encrypted token (decrypt with tokenCipher.open).
	GetCodePlatform(ctx context.Context, id int64) (CodePlatform, []byte, bool, error)
}

// CodePlatformInput is a registration request.
type CodePlatformInput struct {
	Kind    string
	BaseURL string
	Token   string
}

var platformKinds = []string{string(PlatformGitLab), string(PlatformGitHub)}

// tokenTail returns the last 4 characters of a token, or nothing when the token is
// too short for that to be safe to reveal.
func tokenTail(token string) string {
	r := []rune(token)
	if len(r) < 12 {
		return ""
	}
	return string(r[len(r)-4:])
}

// normalizeBaseURL checks an instance address and returns it without a trailing slash.
func normalizeBaseURL(raw string) (string, bool) {
	raw = strings.TrimSpace(raw)
	u, err := url.Parse(raw)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Hostname() == "" ||
		u.User != nil || u.RawQuery != "" || u.Fragment != "" || u.ForceQuery {
		return "", false
	}
	return strings.TrimRight(raw, "/"), true
}

// CreateCodePlatform registers a code platform. Administrators only.
func (s *Service) CreateCodePlatform(ctx context.Context, actor Actor, in CodePlatformInput) (CodePlatform, error) {
	if err := requirePermission(actor, PermPlatformManage); err != nil {
		return CodePlatform{}, err
	}
	var c Collector
	if c.Required("kind", in.Kind) && !slices.Contains(platformKinds, in.Kind) {
		c.Add("kind", CodeInvalid, map[string]any{"allowed": platformKinds})
	}
	base := ""
	if c.Required("baseUrl", in.BaseURL) {
		var ok bool
		if base, ok = normalizeBaseURL(in.BaseURL); !ok {
			c.Add("baseUrl", CodeInvalid, nil)
		}
	}
	if in.Token == "" {
		c.Add("token", CodeRequired, nil)
	}
	if err := c.Err(); err != nil {
		return CodePlatform{}, err
	}
	enc, err := s.tokens.seal([]byte(in.Token))
	if err != nil {
		return CodePlatform{}, err
	}
	p := CodePlatform{Kind: PlatformKind(in.Kind), BaseURL: base, TokenTail: tokenTail(in.Token), CreatedAt: s.nowUTC()}
	id, err := s.repo.CreateCodePlatform(ctx, p, enc)
	if err != nil {
		return CodePlatform{}, err
	}
	p.ID = id
	return p, nil
}

// ListCodePlatforms returns every registered code platform, oldest first. Administrators only.
func (s *Service) ListCodePlatforms(ctx context.Context, actor Actor) ([]CodePlatform, error) {
	if err := requirePermission(actor, PermPlatformManage); err != nil {
		return nil, err
	}
	return s.repo.ListCodePlatforms(ctx)
}
