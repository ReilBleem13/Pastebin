package myerrors

import "errors"

var (
	ErrWrongPassword = errors.New("password is wrong")

	ErrInvalidEmailFormat = errors.New("invalid email format")
	ErrShortPassword      = errors.New("password is too short. Minimum lenght = 10")
	ErrUserNotFound       = errors.New("user not found")
	ErrUserAlreadyExist   = errors.New("user already exists")

	ErrUnexpectedSignMethod = errors.New("unexpected signing method")
	ErrTokenExpired         = errors.New("token expired")
	ErrInvalidToken         = errors.New("token is invalid")

	ErrInternal = errors.New("internal server error")
)
