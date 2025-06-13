package auth

import "time"

// User представляет сущность пользователя
type User struct {
	ID        int
	Email     string
	Password  string
	CreatedAt time.Time
	UpdatedAt time.Time
}

// Token представляет токен доступа
type Token struct {
	AccessToken string
	ExpiresAt   time.Time
}
