package models

import (
	"time"

	"golang.org/x/net/context"
)

type User struct {
	ID           int       `json:"id" db:"id"`
	Name         string    `json:"name" db:"name"`
	Email        string    `json:"email" db:"email"`
	PasswordHash string    `json:"-" db:"password_hash"`
	CreatedAt    time.Time `json:"created_at" db:"created_at"`
}

type Pasta struct {
	ID       int    `json:"-"`
	Hash     string `json:"hash,omitempty" db:"hash"`
	ObjectID string `json:"object_id,omitempty" db:"object_id"`
	UserID   int    `json:"user_id,omitempty" db:"user_id"`

	Size         int    `json:"size" db:"size"`
	Language     string `json:"language" db:"language"`
	Visibility   string `json:"visibility" db:"visibility"`
	Views        int    `json:"views,omitempty" db:"views"`
	PasswordHash string `json:"-" db:"password_hash"`

	ExpireAfterRead bool `json:"expire_after_read" db:"expire_after_read"`

	CreatedAt time.Time `json:"created_at" db:"created_at"`
	ExpiresAt time.Time `json:"expires_at" db:"expires_at"`
}

type PastaWithData struct {
	Text     string `json:"text"`
	Metadata *Pasta `json:"metadata,omitempty"`
}

type PastaPaginated struct {
	Number int    `json:"#"`
	Pasta  string `json:"Pasta"`
}

type PaginateFunc func(ctx context.Context, limit int, offset int, userID int) ([]string, error)
