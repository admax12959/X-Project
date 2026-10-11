package mysqlstore

import (
	"context"

	"github.com/admax12959/X-Project/internal/domain"
)

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
