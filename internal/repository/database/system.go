package database

import (
	"context"

	"github.com/jmoiron/sqlx"
)

type SystemPostgres struct {
	db *sqlx.DB
}

func NewSystemPostgres(db *sqlx.DB) *SystemPostgres {
	return &SystemPostgres{db: db}
}

func (s *SystemPostgres) DeleteExriredMetadata(ctx context.Context) (int, error) {
	var deletedCount int
	err := s.db.QueryRowContext(ctx, `
		WITH to_delete AS (
			SELECT ctid FROM pastas
			WHERE expires_at < now()
			ORDER BY expires_at
			LIMIT 10000
		),
		del AS (
			DELETE FROM pastas
			WHERE ctid IN (SELECT ctid FROM to_delete)
			RETURNING 1
		)
		SELECT count(*) FROM del;`).
		Scan(&deletedCount)
	if err != nil {
		return 0, err
	}
	return deletedCount, nil
}
