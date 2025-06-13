package auth

import "context"

// Service определяет интерфейс сервиса для работы с аутентификацией
type Service interface {
	// Register регистрирует нового пользователя
	Register(ctx context.Context, dto RegisterDTO) error

	// Login выполняет вход пользователя
	Login(ctx context.Context, dto LoginDTO) (*Token, error)

	// ValidateToken проверяет валидность токена
	ValidateToken(ctx context.Context, token string) (*User, error)
}

// RegisterDTO представляет данные для регистрации
type RegisterDTO struct {
	Email    string
	Password string
}

// LoginDTO представляет данные для входа
type LoginDTO struct {
	Email    string
	Password string
}
