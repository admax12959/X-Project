package mysqlstore

import (
	"context"
	"errors"
	"strings"

	"github.com/go-sql-driver/mysql"

	"github.com/admax12959/X-Project/internal/domain"
)

// mapUsernameConflict turns a duplicate-key error on the username or on the local
// login name into domain.ErrUsernameTaken. Other errors pass through.
func mapUsernameConflict(err error) error {
	var me *mysql.MySQLError
	if errors.As(err, &me) && me.Number == 1062 &&
		(strings.Contains(me.Message, "uq_users_username") || strings.Contains(me.Message, "uq_identity_login")) {
		return domain.ErrUsernameTaken
	}
	return err
}

func (s *Store) ListUsers(ctx context.Context) ([]domain.UserWithIdentities, error) {
	rows, err := s.q.QueryContext(ctx, "SELECT "+userCols+" FROM users ORDER BY id")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []domain.UserWithIdentities{}
	index := map[domain.UserID]int{}
	for rows.Next() {
		u, err := scanUser(rows)
		if err != nil {
			return nil, err
		}
		index[u.ID] = len(out)
		out = append(out, domain.UserWithIdentities{User: u, Identities: []domain.LoginIdentity{}})
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	idRows, err := s.q.QueryContext(ctx, "SELECT user_id, kind, login_name, bound_at, bound_via FROM login_identities ORDER BY id")
	if err != nil {
		return nil, err
	}
	defer idRows.Close()
	for idRows.Next() {
		var i domain.LoginIdentity
		var kind, via string
		if err := idRows.Scan(&i.UserID, &kind, &i.LoginName, &i.BoundAt, &via); err != nil {
			return nil, err
		}
		i.Kind, i.BoundVia = domain.IdentityKind(kind), domain.BoundVia(via)
		if at, ok := index[i.UserID]; ok {
			out[at].Identities = append(out[at].Identities, i)
		}
	}
	return out, idRows.Err()
}
