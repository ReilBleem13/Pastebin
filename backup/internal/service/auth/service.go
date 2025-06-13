package auth

import (
	"context"
	"fmt"
	"pastebin/backup/internal/domain/auth"
	"pastebin/backup/internal/domain/events"
	"pastebin/internal/utils"
	"strings"
	"time"
)

type authService struct {
	repo auth.Repository
	bus  *events.EventBus
}

// NewAuthService создает новый экземпляр сервиса аутентификации
func NewAuthService(repo auth.Repository, bus *events.EventBus) auth.Service {
	return &authService{
		repo: repo,
		bus:  bus,
	}
}

func (s *authService) Register(ctx context.Context, dto auth.RegisterDTO) error {
	// Валидация email
	if !strings.Contains(dto.Email, "@") {
		return fmt.Errorf("invalid email format")
	}

	// Валидация пароля
	if len(dto.Password) < 10 {
		return fmt.Errorf("password is too short, minimum length is 10")
	}

	// Хеширование пароля
	hashedPassword, err := utils.HashPassword(dto.Password)
	if err != nil {
		return fmt.Errorf("failed to hash password: %v", err)
	}

	// Создание пользователя
	user := &auth.User{
		Email:     dto.Email,
		Password:  hashedPassword,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	if err := s.repo.CreateUser(ctx, user); err != nil {
		return err
	}

	// Публикуем событие регистрации пользователя
	event := events.UserRegisteredEvent{
		UserID:    user.ID,
		Email:     user.Email,
		CreatedAt: user.CreatedAt,
	}
	if err := s.bus.Publish(ctx, event); err != nil {
		// Логируем ошибку, но не прерываем операцию
		fmt.Printf("Failed to publish user.registered event: %v\n", err)
	}

	return nil
}

func (s *authService) Login(ctx context.Context, dto auth.LoginDTO) (*auth.Token, error) {
	// Получение пользователя
	user, err := s.repo.GetUserByEmail(ctx, dto.Email)
	if err != nil {
		return nil, fmt.Errorf("user not found")
	}

	// Проверка пароля
	if !utils.CheckPasswordHash(dto.Password, user.Password) {
		return nil, fmt.Errorf("invalid password")
	}

	// Генерация токена
	token, err := utils.GenerateToken(user.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to generate token: %v", err)
	}

	return &auth.Token{
		AccessToken: token,
		ExpiresAt:   time.Now().Add(24 * time.Hour),
	}, nil
}

func (s *authService) ValidateToken(ctx context.Context, token string) (*auth.User, error) {
	// Валидация токена
	claims, err := utils.VerifyAccessToken(token)
	if err != nil {
		return nil, fmt.Errorf("invalid token: %v", err)
	}

	// Получение пользователя
	user, err := s.repo.GetUserByID(ctx, claims.UserID)
	if err != nil {
		return nil, fmt.Errorf("user not found")
	}

	return user, nil
}
