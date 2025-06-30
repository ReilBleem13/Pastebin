package domain

import (
	"context"
	"pastebin/pkg/dto"
)

type Authorization interface {
	CreateNewUser(ctx context.Context, user *dto.RequestNewUser) error
	CheckLogin(ctx context.Context, request *dto.LoginUser) error
	GenerateToken(ctx context.Context, request *dto.LoginUser) (string, error)
}
