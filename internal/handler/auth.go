package handler

import (
	"errors"
	customerrors "pastebin/internal/errors"
	"pastebin/internal/utils"
	"pastebin/pkg/dto"
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

	hashPassword, err := utils.HashPassword(request.Password)
	if err != nil {
		h.logger.Errorf("internal server error during hashing password: %v", err)
		c.JSON(500, gin.H{"error": "internal server error"})
		return
	}

	request.Password = hashPassword
	if err := h.servises.Authorization.CreateNewUser(ctx, &request); err != nil {
		if errors.Is(err, customerrors.ErrUserAlreadyExist) {
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
	if err := h.servises.Authorization.CheckLogin(ctx, &request); err != nil {
		if errors.Is(err, customerrors.ErrUserNotFound) {
			c.JSON(404, gin.H{"error": "user not found"})
		} else if errors.Is(err, customerrors.ErrWrongPassword) {
			c.JSON(400, gin.H{"error": "wrong password"})
		} else {
			h.logger.Errorf("internal server error during authorizaton: %v", err)
			c.JSON(500, gin.H{"error": "internal server error"})
		}
		return
	}

	accessToken, err := h.servises.Authorization.GenerateToken(ctx, &request)
	if err != nil {
		h.logger.Errorf("internal server error during generating token: %v", err)
		c.JSON(500, gin.H{"error": "internal server error"})
		return
	}
	c.JSON(200, dto.SuccessLoginedDto{
		Status:      200,
		Message:     "Successfully logined",
		AccessToken: accessToken,
	})
}
