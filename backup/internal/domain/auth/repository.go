package auth

import "context"

// Repository определяет интерфейс для работы с хранилищем пользователей
type Repository interface {
	// CreateUser создает нового пользователя
	CreateUser(ctx context.Context, user *User) error

	// GetUserByEmail получает пользователя по email
	GetUserByEmail(ctx context.Context, email string) (*User, error)

	// GetUserByID получает пользователя по ID
	GetUserByID(ctx context.Context, id int) (*User, error)

	// UpdateUser обновляет данные пользователя
	UpdateUser(ctx context.Context, user *User) error
}
