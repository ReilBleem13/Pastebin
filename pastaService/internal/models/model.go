package models

import (
	"time"

	"golang.org/x/net/context"
)

type Visibility string

const (
	VisibilityPublic  Visibility = "public"
	VisibilityPrivate Visibility = "private"
)

func IsValidVisibility(v Visibility) bool {
	switch v {
	case VisibilityPublic, VisibilityPrivate:
		return true
	default:
		return false
	}
}

type User struct {
	ID           int       `json:"id" db:"id"`
	Name         string    `json:"name" db:"name"`
	Email        string    `json:"email" db:"email"`
	PasswordHash string    `json:"-" db:"password_hash"`
	CreatedAt    time.Time `json:"created_at" db:"created_at"`
}

type Pasta struct {
	ID       int    `json:"-" db:"id"`
	Hash     string `json:"hash" db:"hash"`
	ObjectID string `json:"object_id" db:"object_id"`
	UserID   int    `json:"user_id,omitempty" db:"user_id"`

	Size            int        `json:"size" db:"size"`
	Language        string     `json:"language" db:"language"`
	Visibility      Visibility `json:"visibility" db:"visibility"`
	Views           int        `json:"views" db:"views"`
	ExpireAfterRead bool       `json:"expire_after_read" db:"expire_after_read"`
	PasswordHash    string     `json:"-" db:"password_hash"`

	CreatedAt time.Time `json:"created_at" db:"created_at"`
	ExpiresAt time.Time `json:"expires_at" db:"expires_at"`
}

type PastaWithData struct {
	Text     string `json:"text"`
	Metadata *Pasta `json:"metadata,omitempty"`
}

type PaginateFunc func(ctx context.Context, limit int, offset int, userID int) ([]string, error)
