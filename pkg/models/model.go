package models

import "time"

type Paste struct {
	ID         int       `json:"-"`
	Hash       string    `json:"hash" db:"hash"`
	UserID     int       `json:"user_id,omitempty" db:"user_id"`
	StorageKey string    `json:"storage_key" db:"storage_key"` // переименовать
	Size       int       `json:"size" db:"size"`
	CreatedAt  time.Time `json:"created_at" db:"created_at"`
	ExpiredAt  time.Time `json:"expired_at" db:"expired_at"`
}

type PasteWithData struct {
	// ID         int       `json:"-"`
	// Hash       string    `json:"hash"`
	// UserID     int       `json:"user_id,omitempty"`
	// StorageKey string    `json:"storage_key"`
	// Size       int       `json:"size"`
	// CreatedAt  time.Time `json:"created_at"`
	// ExpiredAt  time.Time `json:"expired_at"`
	Text     string `json:"text"`
	Metadata Paste  `json:"metadata,omitempty"`
	Hash     string `json:"-"`
	ObjectID string `json:"-"`
}
