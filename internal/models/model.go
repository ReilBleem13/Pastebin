package models

import "time"

type Paste struct {
	ID        int       `json:"-"`
	Hash      string    `json:"hash" db:"hash"`
	UserID    int       `json:"user_id,omitempty" db:"user_id"`
	Key       string    `json:"key" db:"key"`
	Size      int       `json:"size" db:"size"`
	CreatedAt time.Time `json:"created_at" db:"created_at"`
	ExpiredAt time.Time `json:"expired_at" db:"expired_at"`
}

type PasteWithData struct {
	Text     string `json:"text"`
	Metadata Paste  `json:"metadata,omitempty"`
}
