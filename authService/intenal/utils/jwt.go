package utils

import (
	"authService/intenal/config"
	myerrors "authService/intenal/errors"
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type Claims struct {
	UserID int `json:"user_id"`
	jwt.RegisteredClaims
}

var (
	jwtKey     = []byte(config.GetConfig().App.JWTSecret)
	accessTTL  = config.GetConfig().App.JWTAccessTTL
	refreshTTL = config.GetConfig().App.JWTRefreshTTL
)

func GenerateTokens(userID int) (string, string, error) {
	now := time.Now()

	accessClaims := Claims{
		UserID: userID,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(now.Add(accessTTL)),
			IssuedAt:  jwt.NewNumericDate(now),
			Subject:   fmt.Sprintf("%d", userID),
		},
	}
	accessToken := jwt.NewWithClaims(jwt.SigningMethodHS256, accessClaims)
	accessTokenString, err := accessToken.SignedString([]byte(jwtKey))
	if err != nil {
		return "", "", err
	}

	refreshClaims := Claims{
		UserID: userID,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(now.Add(refreshTTL)),
			IssuedAt:  jwt.NewNumericDate(now),
			Subject:   fmt.Sprintf("%d", userID),
		},
	}
	refreshToken := jwt.NewWithClaims(jwt.SigningMethodHS256, refreshClaims)
	refreshTokenString, err := refreshToken.SignedString([]byte(jwtKey))
	if err != nil {
		return "", "", err
	}

	return accessTokenString, refreshTokenString, nil
}

func RefreshAccessToken(refreshToken string) (string, error) {
	claims, err := VerifyRefreshToken(refreshToken)
	if err != nil {
		return "", fmt.Errorf("invalid refresh token: %w", err)
	}

	accessClaims := Claims{
		UserID: claims.UserID,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(accessTTL)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			Subject:   fmt.Sprintf("%d", claims.UserID),
		},
	}
	accessToken := jwt.NewWithClaims(jwt.SigningMethodHS256, accessClaims)
	accessTokenString, err := accessToken.SignedString([]byte(jwtKey))
	if err != nil {
		return "", fmt.Errorf("failed to sign access token: %w", err)
	}

	return accessTokenString, nil
}

func VerifyAccessToken(tokenString string) (*Claims, error) {
	return verifyToken(tokenString)
}

func VerifyRefreshToken(tokenString string) (*Claims, error) {
	return verifyToken(tokenString)
}

func verifyToken(tokenString string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, myerrors.ErrUnexpectedSignMethod
		}
		return []byte((jwtKey)), nil
	})

	if err != nil {
		if errors.Is(err, jwt.ErrTokenExpired) {
			return nil, myerrors.ErrTokenExpired
		}

		if errors.Is(err, jwt.ErrTokenSignatureInvalid) {
			return nil, myerrors.ErrInvalidToken
		}
		return nil, fmt.Errorf("could not parse token: %w", err)
	}

	if claims, ok := token.Claims.(*Claims); ok && token.Valid {
		return claims, nil
	}
	return nil, myerrors.ErrInvalidToken
}
