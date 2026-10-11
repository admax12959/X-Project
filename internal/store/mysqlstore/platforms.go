package mysqlstore

import (
	"context"

	"github.com/admax12959/X-Project/internal/domain"
)

var _ domain.CodePlatformRepository = (*Store)(nil)

func (s *Store) CreateCodePlatform(ctx context.Context, p domain.CodePlatform, tokenEnc []byte) (int64, error) {
	res, err := s.q.ExecContext(ctx,
		"INSERT INTO code_platforms (kind, base_url, token_enc, token_tail, created_at) VALUES (?, ?, ?, ?, ?)",
		string(p.Kind), p.BaseURL, tokenEnc, p.TokenTail, p.CreatedAt)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func (s *Store) ListCodePlatforms(ctx context.Context) ([]domain.CodePlatform, error) {
	rows, err := s.q.QueryContext(ctx, "SELECT id, kind, base_url, token_tail, created_at FROM code_platforms ORDER BY id")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []domain.CodePlatform{}
	for rows.Next() {
		var p domain.CodePlatform
		var kind string
		if err := rows.Scan(&p.ID, &kind, &p.BaseURL, &p.TokenTail, &p.CreatedAt); err != nil {
			return nil, err
		}
		p.Kind = domain.PlatformKind(kind)
		out = append(out, p)
	}
	return out, rows.Err()
}
