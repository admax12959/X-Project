package domain

import (
	"context"
	"errors"
	"regexp"
	"strconv"
	"strings"
)

// repoPathPattern: "group/repo", "group/subgroup/repo" (at least two segments).
var repoPathPattern = regexp.MustCompile(`^[\w.-]+(/[\w.-]+)+$`)

// RepoInput is the client's description of a repository binding. IDs arrive as
// strings (JSON convention) and are parsed here.
type RepoInput struct {
	PlatformID string
	Path       string
}

// RepoBinding is a verified repository: which platform, the path it had when it
// was verified (display only) and the platform's stable project ID (ADR-0001).
type RepoBinding struct {
	Platform  CodePlatform
	Path      string
	ProjectID string
}

// VerifyRepo asks the code platform once whether the repository exists and the
// platform token can see it. Administrators only. It stores nothing; creating an
// application verifies again on the server.
func (s *Service) VerifyRepo(ctx context.Context, actor Actor, in RepoInput) (RepoBinding, error) {
	if err := requirePermission(actor, PermRepoVerify); err != nil {
		return RepoBinding{}, err
	}
	var c Collector
	platformID, path := checkRepoInput(&c, "repo", in)
	if err := c.Err(); err != nil {
		return RepoBinding{}, err
	}
	return s.resolveRepo(ctx, s.repo, "repo", platformID, path)
}

// checkRepoInput validates the shape of a repo input and returns the parsed
// platform ID and the normalised path. field is the prefix for error fields.
func checkRepoInput(c *Collector, field string, in RepoInput) (platformID int64, path string) {
	if c.Required(field+".platformId", in.PlatformID) {
		n, err := strconv.ParseInt(strings.TrimSpace(in.PlatformID), 10, 64)
		if err != nil || n <= 0 {
			c.Add(field+".platformId", CodeInvalid, nil)
		} else {
			platformID = n
		}
	}
	path = strings.Trim(strings.TrimSpace(in.Path), "/")
	if c.Required(field+".path", path) && !repoPathPattern.MatchString(path) {
		c.Add(field+".path", CodeInvalid, nil)
		path = ""
	}
	return platformID, path
}

// resolveRepo calls the code platform adapter with the platform's decrypted token
// and maps every non-found outcome to its own error code. platformID and path must
// come from checkRepoInput.
func (s *Service) resolveRepo(ctx context.Context, repo Repository, field string, platformID int64, path string) (RepoBinding, error) {
	platform, tokenEnc, found, err := repo.GetCodePlatform(ctx, platformID)
	if err != nil {
		return RepoBinding{}, err
	}
	if !found {
		return RepoBinding{}, Validation(Invalid(field+".platformId", CodeInvalid, nil))
	}
	token, err := s.tokens.open(tokenEnc)
	if err != nil {
		return RepoBinding{}, err
	}
	if s.codePlatforms == nil {
		return RepoBinding{}, errors.New("no code platform adapter configured")
	}
	res := s.codePlatforms.ResolveRepo(ctx, PlatformConn{Kind: platform.Kind, BaseURL: platform.BaseURL, Token: string(token)}, path)
	switch res.Outcome {
	case RepoFound:
		return RepoBinding{Platform: platform, Path: path, ProjectID: res.ProjectID}, nil
	case RepoNotFound:
		return RepoBinding{}, Validation(Invalid(field+".path", CodeRepoNotFound, nil))
	case RepoForbidden:
		return RepoBinding{}, Validation(Invalid(field+".path", CodeRepoForbidden, nil))
	case PlatformUnreachable:
		// Like the directory, an unreachable platform is 503, not a validation failure.
		return RepoBinding{}, &Error{Kind: KindUnavailable, Errors: []FieldError{Invalid(field+".platformId", CodePlatformUnreachable, nil)}}
	}
	return RepoBinding{}, errors.New("code platform adapter returned an unknown outcome")
}
