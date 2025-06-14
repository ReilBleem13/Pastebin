package models

import "time"

type User struct {
	ID           int       `json:"id" db:"id"`
	Name         string    `json:"name" db:"name"`
	Email        string    `json:"email" db:"email"`
	PasswordHash string    `json:"-" db:"password_hash"`
	CreatedAt    time.Time `json:"created_at" db:"created_at"`
}

type Paste struct {
	ID           int       `json:"-"`
	Hash         string    `json:"hash,omitempty" db:"hash"`
	Key          string    `json:"key,omitempty" db:"key"`
	UserID       int       `json:"user_id,omitempty" db:"user_id"`
	Size         int       `json:"size" db:"size"`
	Language     *string   `json:"language" db:"language"`
	Visibility   *string   `json:"visibility" db:"visibility"`
	Views        int       `json:"views,omitempty" db:"views"`
	PasswordHash string    `json:"-" db:"password_hash"`
	CreatedAt    time.Time `json:"created_at" db:"created_at"`
	ExpiresAt    time.Time `json:"expires_at" db:"expires_at"`
}

type PasteWithData struct {
	Text     string `json:"text"`
	Metadata Paste  `json:"metadata,omitempty"`
}

type PastaPaginated struct {
	Number int    `json:"#"`
	Pasta  string `json:"Pasta"`
}

/*
1. Поиск по ключевым словам
2. Фоновое удаление просроченныз паст с postgres и minio
3. Транзакции!
*/
