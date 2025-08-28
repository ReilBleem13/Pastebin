package domain

import (
	"authService/pkg/dto"
	"context"
)

type AuthRepository interface {
	CreateUser(ctx context.Context, user *dto.RequestNewUser) error
	GetHashPassword(ctx context.Context, email string) (string, error)
	GetUserIDByEmail(ctx context.Context, email string) (int, error)
}
