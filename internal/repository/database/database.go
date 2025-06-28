package database

import (
	"pastebin/internal/domain"

	"github.com/jmoiron/sqlx"
)

type Database struct {
	auth  domain.AuthDatabase
	pasta domain.PastaDatabase
}

func NewDatabase(db *sqlx.DB) *Database {
	return &Database{
		auth:  NewAuthDatabase(db),
		pasta: NewPastaDatabase(db),
	}
}

func (d *Database) Auth() domain.AuthDatabase {
	return d.auth
}

func (d *Database) Pasta() domain.PastaDatabase {
	return d.pasta
}
