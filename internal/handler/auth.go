package handler

import (
	"context"
	"pastebin/internal/utils"
	"pastebin/pkg/dto"

	"github.com/gin-gonic/gin"
)

func (h *Handler) SignUp(c *gin.Context) {
	var request dto.RequestNewUser
	ctx := context.Background()

	if err := c.BindJSON(&request); err != nil {
		c.JSON(400, gin.H{
			"error": err,
		})
		return
	}

	hashPassword, err := utils.HashPassword(request.Password)
	if err != nil {
		c.JSON(400, gin.H{
			"error": err,
		})
		return
	}
	request.Password = hashPassword
	if err := h.servises.Authorization.CreateNewUser(ctx, &request); err != nil {
		c.JSON(400, gin.H{
			"error": err,
		})
	}

	c.JSON(201, gin.H{
		"status": "created!",
	})
}

func (h *Handler) SignIn(c *gin.Context) {
	ctx := context.Background()
	var request dto.LoginUser
	if err := c.BindJSON(&request); err != nil {
		c.JSON(400, gin.H{"error": err})
		return
	}
	if err := h.servises.Authorization.CheckLogin(ctx, &request); err != nil {
		c.JSON(400, gin.H{"error": err})
		return
	}
	accessToken, err := h.servises.Authorization.GenerateToken(ctx, &request)
	if err != nil {
		c.JSON(400, gin.H{"error": err})
		return
	}

	c.JSON(200, gin.H{
		"status":      "loggined",
		"accessToken": accessToken,
	})
}
