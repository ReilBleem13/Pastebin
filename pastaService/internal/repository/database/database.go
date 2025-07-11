package database

import (
	domain "pastebin/internal/domain/repository"

	"github.com/jmoiron/sqlx"
)

const (
	usersTables  = "users"
	pastasTables = "pastas"
)

type Database struct {
	pasta domain.PastaDatabase
}

func NewDatabase(db *sqlx.DB) *Database {
	return &Database{
		pasta: NewPastaDatabase(db),
	}
}

func (d *Database) Pasta() domain.PastaDatabase {
	return d.pasta
}
