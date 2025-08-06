package domain

import (
	"authService/pkg/dto"
	"context"
)

type Authorization interface {
	CreateNewUser(ctx context.Context, user *dto.RequestNewUser) error
	CheckLogin(ctx context.Context, request *dto.LoginUser) error
	GenerateToken(ctx context.Context, request *dto.LoginUser) (string, string, error)
	RefreshAccessToken(refreshToken string) (string, error)
}
