package domain

import (
	"context"
	"errors"
	"slices"
	"strconv"
	"strings"
	"time"
)

// UserRef is a user as shown inside other records (Owner/PE lists, history).
type UserRef struct {
	ID          UserID
	Username    string
	DisplayName string
	Status      UserStatus
}

func refOf(u User) UserRef {
	return UserRef{ID: u.ID, Username: u.Username, DisplayName: u.DisplayName, Status: u.Status}
}

// Application is one maintained application (GLOSSARY: 应用).
// MainBranch is stored once, on the mainline row of the lines table, so the
// application and its mainline can never disagree.
type Application struct {
	ID          int64
	Key         string
	Name        string
	Description string
	Repo        RepoBinding
	MainBranch  string
	Owners      []UserRef
	PEs         []UserRef
	DeployInfo  string
	CreatedAt   time.Time
}

// ApplicationSummary is one row of the application list.
type ApplicationSummary struct {
	Key         string
	Name        string
	Description string
	Repo        RepoBinding
	Owners      []UserRef
	// LineCount is the number of maintenance lines in any status; the mainline is not counted.
	LineCount int
	// LatestVersion is the version with the newest release date on any line (ties:
	// registered last), nil when none is registered. Only ID, Number, ReleasedAt and
	// Line (ID, IsMain, Name, Branch) are filled.
	LatestVersion *Version
}

// Line is the mainline or a maintenance line (GLOSSARY: 线). The mainline has no
// name of its own and none of the maintenance-line fields below: no status, source version,
// reason, planned end or time of forking. Lines read for a Version (Version.Line) carry
// only ID, IsMain, Name and Branch.
type Line struct {
	ID            int64
	IsMain        bool
	Name          string
	Branch        string
	CreatedAt     time.Time
	Status        LineStatus     // maintenance lines only
	SourceVersion *SourceVersion // maintenance lines only; fixed at fork time
	Reason        string         // fork reason, fixed at fork time
	PlannedEnd    *time.Time     // optional, informational only
	Note          string
	ForkedAt      *time.Time // set by the server when forked
}

// MemberRole is a user's role on one application.
type MemberRole string

const (
	RoleOwner MemberRole = "owner"
	RolePE    MemberRole = "pe"
)

// NewApplication is what the repository stores for a created application.
type NewApplication struct {
	Key         string
	Name        string
	Description string
	PlatformID  int64
	RepoPath    string
	ProjectID   string
	DeployInfo  string
	At          time.Time
}

// ErrKeyTaken is returned by the Repository when the application key already exists.
var ErrKeyTaken = errors.New("application key already taken")

// ApplicationRepository is the application part of Repository.
type ApplicationRepository interface {
	// CreateApplication inserts the application row. Returns ErrKeyTaken on a duplicate key.
	CreateApplication(ctx context.Context, a NewApplication) (int64, error)
	// CreateMainline inserts the application's mainline (is_main) with the given branch.
	CreateMainline(ctx context.Context, applicationID int64, branch string, at time.Time) error
	// SetMembers stores the Owner or PE list in order.
	SetMembers(ctx context.Context, applicationID int64, role MemberRole, users []UserID) error
	// ListLines returns the application's lines, mainline first.
	ListLines(ctx context.Context, applicationID int64) ([]Line, error)
	GetApplicationByKey(ctx context.Context, key string) (Application, bool, error)
	// ListApplications returns applications ordered by key. A non-empty Query keeps
	// those whose key, name, repository path or an Owner's name contains it
	// (case-insensitive, matched literally). A non-zero MemberID keeps those where
	// that user is an Owner or a PE. Both conditions must hold.
	ListApplications(ctx context.Context, f ApplicationFilter) ([]ApplicationSummary, error)
}

// ApplicationFilter narrows the application list.
type ApplicationFilter struct {
	Query    string
	MemberID UserID // 0 = no restriction
}

// CreateApplicationInput is the administrator's request to create an application.
type CreateApplicationInput struct {
	Key         string
	Name        string
	Description string
	Repo        RepoInput
	MainBranch  string // empty = "main"
	Owners      []string
	PEs         []string
}

// DefaultMainBranch is the mainline branch name when none is given.
const DefaultMainBranch = "main"

// CreateApplication creates an application together with its mainline and the
// first change-history event. Administrators only. The repository is resolved
// through the code platform adapter here, on the server; the client never
// supplies the project ID, so an application cannot exist without a passing
// verification.
func (s *Service) CreateApplication(ctx context.Context, actor Actor, in CreateApplicationInput) (Application, error) {
	if err := requireAdmin(actor); err != nil {
		return Application{}, err
	}
	var c Collector
	key := strings.TrimSpace(in.Key)
	name := strings.TrimSpace(in.Name)
	description := strings.TrimSpace(in.Description)
	branch := strings.TrimSpace(in.MainBranch)
	if branch == "" {
		branch = DefaultMainBranch
	}
	if c.Required("key", key) && !applicationKeyPattern.MatchString(key) {
		c.Add("key", CodeInvalid, nil)
	}
	if c.Required("name", name) {
		checkLength(&c, "name", name, maxNameLength)
	}
	checkLength(&c, "description", description, maxDescriptionLength)
	if !validBranchName(branch) {
		c.Add("mainBranch", CodeInvalid, nil)
	}
	platformID, path := checkRepoInput(&c, "repo", in.Repo)
	owners := parseUserIDs(&c, "owners", in.Owners)
	pes := parseUserIDs(&c, "pes", in.PEs)
	if len(in.Owners) == 0 {
		c.Add("owners", CodeRequired, nil)
	}
	if err := c.Err(); err != nil {
		return Application{}, err
	}

	// The (slow, remote) platform call stays outside the transaction.
	binding, err := s.resolveRepo(ctx, s.repo, "repo", platformID, path)
	if err != nil {
		return Application{}, err
	}
	var created Application
	err = s.repo.InTx(ctx, func(tx Repository) error {
		users, err := s.lockMembers(ctx, tx, owners, pes)
		if err != nil {
			return err
		}
		if err := checkMembers(users, owners, nil, pes, nil); err != nil {
			return err
		}
		now := s.nowUTC()
		id, err := tx.CreateApplication(ctx, NewApplication{
			Key: key, Name: name, Description: description, PlatformID: binding.Platform.ID,
			RepoPath: binding.Path, ProjectID: binding.ProjectID, At: now,
		})
		if errors.Is(err, ErrKeyTaken) {
			return Validation(Invalid("key", CodeKeyTaken, nil))
		}
		if err != nil {
			return err
		}
		if err := tx.CreateMainline(ctx, id, branch, now); err != nil {
			return err
		}
		if err := tx.SetMembers(ctx, id, RoleOwner, owners); err != nil {
			return err
		}
		if err := tx.SetMembers(ctx, id, RolePE, pes); err != nil {
			return err
		}
		if err := s.recordEvent(ctx, tx, id, actor, appCreatedEvent(owners, pes)); err != nil {
			return err
		}
		var found bool
		created, found, err = tx.GetApplicationByKey(ctx, key)
		if err == nil && !found {
			err = errors.New("created application not found")
		}
		return err
	})
	return created, err
}

const maxDescriptionLength = 4000

// lockMembers reads every person named in the lists under a shared lock (in ID order), so
// nobody can be disabled until the transaction ends. Take it before locking the
// application row: disabling a user locks the user first and the applications second.
func (s *Service) lockMembers(ctx context.Context, tx Repository, lists ...[]UserID) (map[UserID]User, error) {
	var ids []UserID
	for _, l := range lists {
		for _, id := range l {
			if !slices.Contains(ids, id) {
				ids = append(ids, id)
			}
		}
	}
	slices.Sort(ids)
	return tx.ReadUsersLocked(ctx, ids)
}

// checkMembers applies the Owner/PE rules to the final lists. users is what lockMembers
// returned. Anyone newly added (not in oldOwners / oldPEs) must be an existing enabled
// user (an LDAP user must log in once first); someone already listed may stay even if
// disabled since. After the change at least one enabled Owner must remain.
func checkMembers(users map[UserID]User, owners, oldOwners, pes, oldPEs []UserID) error {
	var c Collector
	for _, list := range []struct {
		field    string
		ids, old []UserID
	}{{"owners", owners, oldOwners}, {"pes", pes, oldPEs}} {
		for _, id := range without(list.ids, list.old) {
			if u, found := users[id]; !found || u.Status != UserEnabled {
				c.Add(list.field, CodeUserUnavailable, map[string]any{"userId": strconv.FormatInt(int64(id), 10)})
			}
		}
	}
	if c.Err() == nil && !slices.ContainsFunc(owners, func(id UserID) bool { return users[id].Status == UserEnabled }) {
		c.Add("owners", CodeNoEnabledOwner, nil)
	}
	return c.Err()
}

// parseUserIDs parses client-supplied user IDs, dropping duplicates.
func parseUserIDs(c *Collector, field string, raw []string) []UserID {
	var out []UserID
	seen := map[UserID]bool{}
	for _, r := range raw {
		n, err := strconv.ParseInt(strings.TrimSpace(r), 10, 64)
		if err != nil || n <= 0 {
			c.Add(field, CodeInvalid, nil)
			continue
		}
		if id := UserID(n); !seen[id] {
			seen[id] = true
			out = append(out, id)
		}
	}
	return out
}

// ApplicationListQuery is the list request as sent. Mine is the raw query string:
// "" and "false" list everything, "true" only the applications where the current
// user is an Owner or a PE.
type ApplicationListQuery struct {
	Query string
	Mine  string
}

// ListApplications returns applications matching the query (empty = all); any logged-in user may browse.
func (s *Service) ListApplications(ctx context.Context, actor Actor, q ApplicationListQuery) ([]ApplicationSummary, error) {
	f := ApplicationFilter{Query: strings.TrimSpace(q.Query)}
	switch strings.TrimSpace(q.Mine) {
	case "", "false":
	case "true":
		f.MemberID = actor.User.ID
	default:
		return nil, Validation(Invalid("mine", CodeInvalid, nil))
	}
	return s.repo.ListApplications(ctx, f)
}

// ListLines returns an application's lines, mainline first; any logged-in user may read them.
func (s *Service) ListLines(ctx context.Context, actor Actor, key string) ([]Line, error) {
	a, err := s.GetApplication(ctx, actor, key)
	if err != nil {
		return nil, err
	}
	return s.repo.ListLines(ctx, a.ID)
}

// GetApplication returns one application by key; any logged-in user may read it.
func (s *Service) GetApplication(ctx context.Context, _ Actor, key string) (Application, error) {
	a, found, err := s.repo.GetApplicationByKey(ctx, key)
	if err != nil {
		return Application{}, err
	}
	if !found {
		return Application{}, newKind(KindNotFound, CodeNotFound)
	}
	return a, nil
}
