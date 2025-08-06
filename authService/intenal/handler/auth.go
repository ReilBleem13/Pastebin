package handler

import (
	customerrors "authService/intenal/errors"
	"authService/pkg/dto"
	"errors"

	"github.com/gin-gonic/gin"
	"github.com/theartofdevel/logging"
)

func (h *Handler) SignUp(c *gin.Context) {
	var request dto.RequestNewUser

	ctx := c.Request.Context()

	if err := c.BindJSON(&request); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	if err := h.servise.CreateNewUser(ctx, &request); err != nil {
		if errors.Is(err, customerrors.ErrUserAlreadyExist) {
			c.JSON(409, gin.H{"error": customerrors.ErrUserAlreadyExist.Error()})
		} else {
			h.logger.Error("Internal server error during creating new user", logging.ErrAttr(err))
			c.JSON(500, gin.H{"error": customerrors.ErrInternal.Error()})
		}
		return
	}

	c.JSON(201, dto.SuccessRegisterDTO{
		Status:  201,
		Message: "Successfully created",
	})
}

func (h *Handler) SignIn(c *gin.Context) {
	ctx := c.Request.Context()
	var request dto.LoginUser

	if err := c.BindJSON(&request); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	if err := h.servise.CheckLogin(ctx, &request); err != nil {
		if errors.Is(err, customerrors.ErrUserNotFound) {
			c.JSON(404, gin.H{"error": customerrors.ErrUserNotFound.Error()})
		} else if errors.Is(err, customerrors.ErrWrongPassword) {
			c.JSON(400, gin.H{"error": customerrors.ErrWrongPassword.Error()})
		} else {
			h.logger.Error("Internal server error during checking login", logging.ErrAttr(err))
			c.JSON(500, gin.H{"error": customerrors.ErrInternal.Error()})
		}
		return
	}

	accessToken, refreshToken, err := h.servise.GenerateToken(ctx, &request)
	if err != nil {
		h.logger.Error("Internal server error during generating token", logging.ErrAttr(err))
		c.JSON(500, gin.H{"error": customerrors.ErrInternal.Error()})
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
	if err := c.BindJSON(&req); err != nil {
		c.JSON(404, gin.H{"error": err.Error()})
	}

	accessToken, err := h.servise.RefreshAccessToken(req.RefreshToken)
	if err != nil {
		if errors.Is(err, customerrors.ErrTokenExpired) {
			c.JSON(401, gin.H{"error": "refresh token expired"})
		} else if errors.Is(err, customerrors.ErrInvalidToken) ||
			errors.Is(err, customerrors.ErrUnexpectedSignMethod) {
			c.JSON(401, gin.H{"error": "invalid refresh token"})
		} else {
			c.JSON(500, gin.H{"error": customerrors.ErrInternal.Error()})
		}
		return
	}
	c.JSON(200, dto.TokenResponse{AccessToken: accessToken})
}
