package handler

import (
	myerrors "authService/intenal/errors"
	"authService/pkg/dto"
	"authService/pkg/hash"
	"authService/pkg/jwt"
	"errors"
	"time"

	"github.com/gin-gonic/gin"
)

func (h *Handler) SignUp(c *gin.Context) {
	start := time.Now()
	defer func() {
		h.logger.Tracef("func() SignUp. Execution time: %.2f", time.Since(start).Seconds())
	}()

	var request dto.RequestNewUser
	ctx := c.Request.Context()

	if err := c.BindJSON(&request); err != nil {
		c.JSON(400, gin.H{"error": "bad request"})
		return
	}

	hashPassword, err := hash.HashPassword(request.Password)
	if err != nil {
		h.logger.Errorf("internal server error during hashing password: %v", err)
		c.JSON(500, gin.H{"error": "internal server error"})
		return
	}

	request.Password = hashPassword
	if err := h.servise.CreateNewUser(ctx, &request); err != nil {
		if errors.Is(err, myerrors.ErrUserAlreadyExist) {
			c.JSON(409, gin.H{"error": "user already exists"})
		} else {
			c.JSON(500, gin.H{"error": "internal server error"})
		}
		return
	}

	c.JSON(201, dto.SuccessRegisterDTO{
		Status:  201,
		Message: "Successfully created",
	})
}

func (h *Handler) SignIn(c *gin.Context) {
	start := time.Now()
	defer func() {
		h.logger.Tracef("func() SignIn. Execution time: %.2f", time.Since(start).Seconds())
	}()

	ctx := c.Request.Context()
	var request dto.LoginUser

	if err := c.BindJSON(&request); err != nil {
		c.JSON(400, gin.H{"error": "bad request"})
		return
	}
	if err := h.servise.CheckLogin(ctx, &request); err != nil {
		if errors.Is(err, myerrors.ErrUserNotFound) {
			c.JSON(404, gin.H{"error": "user not found"})
		} else if errors.Is(err, myerrors.ErrWrongPassword) {
			c.JSON(400, gin.H{"error": "wrong password"})
		} else {
			h.logger.Errorf("internal server error during authorizaton: %v", err)
			c.JSON(500, gin.H{"error": "internal server error"})
		}
		return
	}

	accessToken, refreshToken, err := h.servise.GenerateToken(ctx, &request)
	if err != nil {
		h.logger.Errorf("internal server error during generating token: %v", err)
		c.JSON(500, gin.H{"error": "internal server error"})
		return
	}
	c.JSON(200, dto.SuccessLoginedDto{
		Status:       200,
		Message:      "Successfully logined",
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
	})
}

func (h *Handler) RefreshTokenHandler(c *gin.Context) {
	var req dto.RefreshRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(404, gin.H{"error": "refresh_token required"})
	}

	accessToken, err := jwt.RefreshAccessToken(req.RefreshToken)
	if err != nil {
		switch err {
		case myerrors.ErrTokenExpired:
			c.JSON(401, gin.H{"error": "refresh token expired"})
		case myerrors.ErrInvalidToken, myerrors.ErrUnexpectedSignMethod:
			c.JSON(401, gin.H{"error": "invalid refresh token"})
		default:
			c.JSON(500, gin.H{"error": "internal server error"})
		}
		return
	}
	c.JSON(200, dto.TokenResponse{AccessToken: accessToken})
}
