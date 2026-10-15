// Package fakes provides in-memory test doubles for the domain's adapter ports.
// Tests configure them through apitest.Env.Directory / apitest.Env.CodePlatforms.
package fakes

import (
	"context"
	"sync"

	"github.com/admax12959/X-Project/internal/domain"
)

// Directory is a fake LDAP directory.
type Directory struct {
	mu          sync.Mutex
	accounts    map[string]dirAccount
	unreachable bool
}

type dirAccount struct {
	password string
	entry    domain.DirectoryEntry
	disabled bool
}

var _ domain.Directory = (*Directory)(nil)

func NewDirectory() *Directory { return &Directory{accounts: map[string]dirAccount{}} }

// AddAccount adds or replaces an enabled account. entry.Login is set to login.
func (d *Directory) AddAccount(login, password string, entry domain.DirectoryEntry) {
	d.mu.Lock()
	defer d.mu.Unlock()
	entry.Login = login
	d.accounts[login] = dirAccount{password: password, entry: entry}
}

// SetDisabled marks an existing account as disabled (or enabled again) in the directory.
func (d *Directory) SetDisabled(login string, disabled bool) {
	d.mu.Lock()
	defer d.mu.Unlock()
	a := d.accounts[login]
	a.disabled = disabled
	d.accounts[login] = a
}

// RemoveAccount deletes an account from the directory.
func (d *Directory) RemoveAccount(login string) {
	d.mu.Lock()
	defer d.mu.Unlock()
	delete(d.accounts, login)
}

// SetUnreachable makes every call report DirectoryUnreachable.
func (d *Directory) SetUnreachable(v bool) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.unreachable = v
}

func (d *Directory) Authenticate(_ context.Context, login, password string) domain.DirectoryResult {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.unreachable {
		return domain.DirectoryResult{Outcome: domain.DirectoryUnreachable}
	}
	a, ok := d.accounts[login]
	switch {
	case !ok:
		return domain.DirectoryResult{Outcome: domain.DirectoryNotFound}
	case a.disabled:
		return domain.DirectoryResult{Outcome: domain.DirectoryDisabled}
	case a.password != password:
		return domain.DirectoryResult{Outcome: domain.DirectoryBadPassword}
	}
	return domain.DirectoryResult{Outcome: domain.DirectoryOK, Entry: a.entry}
}

func (d *Directory) Lookup(_ context.Context, login string) domain.DirectoryResult {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.unreachable {
		return domain.DirectoryResult{Outcome: domain.DirectoryUnreachable}
	}
	a, ok := d.accounts[login]
	switch {
	case !ok:
		return domain.DirectoryResult{Outcome: domain.DirectoryNotFound}
	case a.disabled:
		return domain.DirectoryResult{Outcome: domain.DirectoryDisabled}
	}
	return domain.DirectoryResult{Outcome: domain.DirectoryOK, Entry: a.entry}
}

// CodePlatform is a fake GitLab/GitHub. Repositories are keyed by platform base URL and path;
// unknown paths resolve to RepoNotFound.
type CodePlatform struct {
	mu          sync.Mutex
	repos       map[string]domain.RepoResult
	unreachable map[string]bool
}

var _ domain.CodePlatformAdapter = (*CodePlatform)(nil)

func NewCodePlatform() *CodePlatform {
	return &CodePlatform{repos: map[string]domain.RepoResult{}, unreachable: map[string]bool{}}
}

func repoKey(baseURL, path string) string { return baseURL + "\x00" + path }

// AddRepo makes path on the platform at baseURL resolve to projectID.
func (p *CodePlatform) AddRepo(baseURL, path, projectID string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.repos[repoKey(baseURL, path)] = domain.RepoResult{Outcome: domain.RepoFound, ProjectID: projectID}
}

// RenameRepo moves a repository to a new path, keeping its project ID.
func (p *CodePlatform) RenameRepo(baseURL, oldPath, newPath string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	r := p.repos[repoKey(baseURL, oldPath)]
	delete(p.repos, repoKey(baseURL, oldPath))
	p.repos[repoKey(baseURL, newPath)] = r
}

// SetForbidden makes path on the platform resolve to RepoForbidden (token has no access).
func (p *CodePlatform) SetForbidden(baseURL, path string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.repos[repoKey(baseURL, path)] = domain.RepoResult{Outcome: domain.RepoForbidden}
}

// SetUnreachable makes every call to the platform at baseURL report PlatformUnreachable.
func (p *CodePlatform) SetUnreachable(baseURL string, v bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.unreachable[baseURL] = v
}

func (p *CodePlatform) ResolveRepo(_ context.Context, platform domain.PlatformConn, path string) domain.RepoResult {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.unreachable[platform.BaseURL] {
		return domain.RepoResult{Outcome: domain.PlatformUnreachable}
	}
	if r, ok := p.repos[repoKey(platform.BaseURL, path)]; ok {
		return r
	}
	return domain.RepoResult{Outcome: domain.RepoNotFound}
}

// DirectoryFactory is the fake of domain.DirectoryFactory: whatever LDAP configuration the
// application builds a directory for, it returns the one fake Directory, and it records every
// configuration it was asked for (the saved one on each login, the unsaved form on "test
// connection"), so tests can assert what the application would have connected to.
type DirectoryFactory struct {
	dir *Directory

	mu       sync.Mutex
	configs  []domain.LDAPConfig
	buildErr error
}

var _ domain.DirectoryFactory = (*DirectoryFactory)(nil)

func NewDirectoryFactory(dir *Directory) *DirectoryFactory { return &DirectoryFactory{dir: dir} }

func (f *DirectoryFactory) Build(cfg domain.LDAPConfig) (domain.Directory, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.configs = append(f.configs, cfg)
	if f.buildErr != nil {
		return nil, f.buildErr
	}
	return f.dir, nil
}

// Configs returns every configuration a directory was built for, oldest first.
func (f *DirectoryFactory) Configs() []domain.LDAPConfig {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]domain.LDAPConfig(nil), f.configs...)
}

// LastConfig returns the configuration of the latest Build call.
func (f *DirectoryFactory) LastConfig() (domain.LDAPConfig, bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.configs) == 0 {
		return domain.LDAPConfig{}, false
	}
	return f.configs[len(f.configs)-1], true
}

// FailBuild makes Build return err (nil = works again), like an adapter refusing a configuration.
func (f *DirectoryFactory) FailBuild(err error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.buildErr = err
}
