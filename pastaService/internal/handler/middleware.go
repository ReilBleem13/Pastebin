package handler

import (
	"errors"
	"net/http"
	customerrors "pastebin/internal/errors"
	"pastebin/internal/utils"
	"pastebin/pkg/dto"
	"strings"

	"github.com/gin-gonic/gin"
)

const (
	authorizationHeader = "Authorization"
	visibilityPrivate   = "private"
	tokenPrefix         = "Bearer "

	userCtx       = "userId"
	requestCtx    = "request"
	visibilityCtx = "visibility"
)

func (h *Handler) AuthMiddleWare() gin.HandlerFunc {
	return func(c *gin.Context) {
		header := c.GetHeader(authorizationHeader)
		if header == "" || !strings.HasPrefix(header, tokenPrefix) {
			c.Next()
			return
		}

		token := strings.TrimPrefix(header, tokenPrefix)
		claims, err := utils.VerifyAccessToken(token)
		if err != nil {
			if errors.Is(err, customerrors.ErrTokenExpired) {
				c.JSON(401, gin.H{"error": "token has expired"})
				c.Abort()
				return
			}

			if errors.Is(err, customerrors.ErrInvalidToken) ||
				errors.Is(err, customerrors.ErrUnexpectedSignMethod) {
				c.JSON(401, gin.H{"error": "invalid token"})
				c.Abort()
				return
			}

			h.logger.Errorf("unexpected error during token verification: %v", err)
			c.JSON(500, customerrors.ErrInternal)
			c.Abort()
			return
		}

		c.Set(userCtx, claims.UserID)
		c.Next()
	}
}

func (h *Handler) RequireAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		_, exists := c.Get(userCtx)
		if !exists {
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

			if strings.ToLower(req.Visibility) == visibilityPrivate {
				_, exists := c.Get(userCtx)
				if !exists {
					c.JSON(401, gin.H{"error": "unathorized: create private pastas require login"})
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
		ctx := c.Request.Context()

		visibility, err := h.servises.Pasta.GetVisibility(ctx, hash)
		if err != nil {
			if errors.Is(err, customerrors.ErrPastaNotFound) {
				c.JSON(404, gin.H{"error": err.Error()})
			} else {
				h.logger.Errorf("Internal error on GetVisibility: %v", err)
				c.JSON(500, gin.H{"error": err.Error()})
			}
			c.Abort()
			return
		}
		c.Set(visibilityCtx, visibility)

		if visibility == visibilityPrivate {
			_, exists := c.Get(userCtx)
			if !exists {
				c.JSON(401, gin.H{"error": "unauthorized: private pasta"})
				c.Abort()
				return
			}
		}
		c.Next()
	}
}

// получение userID c context
func (h *Handler) GetUserID(c *gin.Context) (int, error) {
	rawUserID, exists := c.Get(userCtx)
	if !exists {
		return 0, customerrors.ErrUserNotAuthenticated
	} //подумать

	userID, ok := rawUserID.(int)
	if !ok {
		h.logger.Errorf("invalid type for userCtx: %T", userID)
		return 0, customerrors.ErrInternal
	}
	return userID, nil
}

// получение request с context
func (h *Handler) GetRequest(c *gin.Context) (*dto.RequestCreatePasta, error) {
	rawRequest, exists := c.Get(requestCtx)
	if !exists {
		h.logger.Error("critical: requestCtx not found in context")
		return nil, customerrors.ErrInternal
	}

	request, ok := rawRequest.(dto.RequestCreatePasta)
	if !ok {
		h.logger.Errorf("invalid type for requestCtx: %T", request)
		return nil, customerrors.ErrInternal
	}
	return &request, nil
}

// получение visibility c context
func (h *Handler) GetVisibility(c *gin.Context) (string, error) {
	rawVisibility, exists := c.Get(visibilityCtx)
	if !exists {
		h.logger.Error("critical: visibilityCtx not found in context")
		return "", customerrors.ErrInternal
	}

	visibility, ok := rawVisibility.(string)
	if !ok {
		h.logger.Errorf("invalid type for requestCtx: %T", visibility)
		return "", customerrors.ErrInternal
	}
	return visibility, nil
}
