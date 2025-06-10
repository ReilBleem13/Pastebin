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
	Hash         string    `json:"hash" db:"hash"`
	Key          string    `json:"key" db:"key"`
	UserID       int       `json:"user_id,omitempty" db:"user_id"`
	Size         int       `json:"size" db:"size"`
	Language     *string   `json:"language" db:"language"`
	Visibility   *string   `json:"visibility" db:"visibility"`
	Views        int       `json:"views" db:"views"`
	PasswordHash string    `json:"-" db:"password_hash"`
	CreatedAt    time.Time `json:"created_at" db:"created_at"`
	ExpiresAt    time.Time `json:"expired_at" db:"expires_at"`
}

type PasteWithData struct {
	Text     string `json:"text"`
	Metadata Paste  `json:"metadata,omitempty"`
}

/*
1. добавить raw
2. логика для пароля
3. фоновое удаление
4. привести код в красивое состояние
5. добавить: удаление, изменение, пагинацию своих паст для юзера
*/
