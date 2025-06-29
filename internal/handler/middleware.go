package handler

import (
	"context"
	"errors"
	"log"
	"net/http"
	"pastebin/internal/utils"
	"pastebin/pkg/dto"
	"strings"

	"github.com/gin-gonic/gin"
)

const (
	authorizationHeader = "Authorization"
	userCtx             = "userId"
	requestCtx          = "request"
	visibilityCtx       = "visibility"
)

func (h *Handler) AuthMiddleWare() gin.HandlerFunc {
	return func(c *gin.Context) {
		header := c.GetHeader(authorizationHeader)
		if header == "" || !strings.HasPrefix(header, "Bearer ") {
			c.Next()
			return
		}

		token := strings.TrimPrefix(header, "Bearer ")
		claims, err := utils.VerifyAccessToken(token)
		if err != nil {
			log.Printf("AuthMiddleWare. Error: %v", err)
			c.Next()
			return
		}

		c.Set(userCtx, claims.UserID)
		c.Next()
	}
}

func (h *Handler) RequireAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		userID, exists := c.Get(userCtx)
		if !exists || userID == nil {
			c.JSON(401, gin.H{"error": "unauthorized"})
			c.Abort()
			return
		}
		c.Next()
	}
}

func (h *Handler) AccessPostMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.Request.Method == http.MethodPost {
			var req dto.RequestCreatePasta
			if err := c.ShouldBindJSON(&req); err != nil {
				c.JSON(400, gin.H{"error": "invalid request"})
				c.Abort()
				return
			}

			c.Set(requestCtx, req)

			if req.Visibility != "" && req.Visibility == "private" {
				userID, exists := c.Get(userCtx)
				if !exists || userID == nil {
					c.JSON(401, gin.H{"error": "unathorized: private pastas require login"})
					c.Abort()
					return
				}
			}
			c.Next()
			return
		}
	}
}

func (h *Handler) AccessByKeyMiddleware() gin.HandlerFunc {
	// временая логика бд
	return func(c *gin.Context) {
		hash := c.Param("objectID")
		ctx := context.Background()
		visibility, err := h.servises.Pasta.GetVisibility(ctx, hash)
		if err != nil {
			c.JSON(400, gin.H{"error": err.Error()})
			c.Abort()
			return
		}

		c.Set(visibilityCtx, visibility)

		if visibility == "private" {
			userID, exists := c.Get(userCtx)
			if !exists || userID == nil {
				c.JSON(401, gin.H{"error": "unauthorized: private pasta"})
				c.Abort()
				return
			}
		}
		c.Next()
	}
}

func (h *Handler) GetUserID(c *gin.Context) (int, error) {
	id, exists := c.Get(userCtx)
	if !exists {
		return 0, nil
	}

	idInt, ok := id.(int)
	if !ok {
		return 0, errors.New("user id is of invalid type")
	}

	return idInt, nil
}

func (h *Handler) GetRequest(c *gin.Context) (*dto.RequestCreatePasta, error) {
	request, exists := c.Get(requestCtx)
	if !exists {
		return nil, errors.New("user id not found")
	}
	requestNew, ok := request.(dto.RequestCreatePasta)
	if !ok {
		return nil, errors.New("user id is of invalid type")
	}

	return &requestNew, nil
}

func (h *Handler) GetVisibility(c *gin.Context) (string, error) {
	visib, exists := c.Get(visibilityCtx)
	if !exists {
		return "", errors.New("visibility not found")
	}
	visibNew, ok := visib.(string)
	if !ok {
		return "", errors.New("visibility is of invalid type")
	}

	return visibNew, nil
}
