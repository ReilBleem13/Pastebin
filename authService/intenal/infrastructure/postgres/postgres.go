package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/jmoiron/sqlx"
)

type PostgresDB struct {
	db *sqlx.DB
}

func NewPostgresDB(ctx context.Context, dbURL string) (*PostgresDB, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	db, err := sqlx.ConnectContext(ctx, "postgres", dbURL)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}
	return &PostgresDB{db: db}, nil
}

func (p *PostgresDB) Close() {
	if p != nil {
		p.db.Close()
	}
}

func (p *PostgresDB) Client() *sqlx.DB {
	return p.db
}
