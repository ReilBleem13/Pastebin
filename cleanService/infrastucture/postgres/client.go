package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/lib/pq"
)

type Postgres interface {
	DeletePastas(ctx context.Context, objectIDs []string) error
}

type PostgresClient struct {
	db *sqlx.DB
}

const batchSize = 1000

func NewPostgresClient(ctx context.Context, dbURL string) (*PostgresClient, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	db, err := sqlx.ConnectContext(ctx, "postgres", dbURL)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}
	return &PostgresClient{db: db}, nil
}

func (p *PostgresClient) DeletePastas(ctx context.Context, objectIDs []string) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}

	tx, err := p.db.BeginTxx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			tx.Rollback()
		} else {
			tx.Commit()
		}
	}()

	_, err = tx.ExecContext(ctx, `CREATE TEMP TABLE tmp_delete_ids (id TEXT PRIMARY KEY) ON COMMIT DROP`)
	if err != nil {
		return fmt.Errorf("create temp table: %w", err)
	}

	for i := 0; i < len(objectIDs); i += batchSize {
		if ctx.Err() != nil {
			return ctx.Err()
		}

		end := i + batchSize
		if end > len(objectIDs) {
			end = len(objectIDs)
		}

		batch := objectIDs[i:end]
		_, err := tx.ExecContext(ctx, "INSERT INTO tmp_delete_ids (id) SELECT unnest($1::text[])", pq.Array(batch))
		if err != nil {
			return fmt.Errorf("insert into temp table: %w", err)
		}
	}

	_, err = tx.ExecContext(ctx,
		`	DELETE FROM pastas
			WHERE object_id IN (SELECT id FROM tmp_delete_ids) 
		`)
	if err != nil {
		return fmt.Errorf("delete from main table: %w", err)
	}
	return nil
}

func (p *PostgresClient) Close() {
	if p != nil {
		p.db.Close()
	}
}

func (p *PostgresClient) Client() *sqlx.DB {
	return p.db
}
