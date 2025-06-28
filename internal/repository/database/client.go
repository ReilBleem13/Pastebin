package database

import (
	"context"
	"fmt"
	"pastebin/internal/config"
	"time"

	"github.com/jmoiron/sqlx"
)

const (
	usersTables  = "users"
	pastasTables = "pastas"
)

type PostgresDB struct {
	db *sqlx.DB
}

func NewPostgresDB(ctx context.Context, cfg config.StorageConfig) (*PostgresDB, error) {
	dsn := fmt.Sprintf("host=%s port=%s user=%s dbname=%s password=%s sslmode=%s",
		cfg.Host, cfg.Port, cfg.Username, cfg.Dbname, cfg.Password, cfg.Sslmode)

	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	db, err := sqlx.ConnectContext(ctx, "postgres", dsn)
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
