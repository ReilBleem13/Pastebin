package handler

import (
	"errors"
	"net/http"
	customerrors "pastebin/internal/errors"
	"pastebin/internal/utils"
	"pastebin/pkg/dto"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/theartofdevel/logging"
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
		h.logger.Debug("Calling AuthMiddleware")
		header := c.GetHeader(authorizationHeader)
		if header == "" || !strings.HasPrefix(header, tokenPrefix) {
			c.Next()
			return
		}

		token := strings.TrimPrefix(header, tokenPrefix)
		claims, err := utils.VerifyAccessToken(token)
		if err != nil {
			if errors.Is(err, customerrors.ErrTokenExpired) {
				c.JSON(401, gin.H{"error": customerrors.ErrTokenExpired.Error()})
				c.Abort()
				return
			}

			if errors.Is(err, customerrors.ErrInvalidToken) ||
				errors.Is(err, customerrors.ErrUnexpectedSignMethod) {
				c.JSON(401, gin.H{"error": customerrors.ErrInvalidToken.Error()})
				c.Abort()
				return
			}

			h.logger.Error("Unexpected error during token verification", logging.ErrAttr(err))
			c.JSON(500, gin.H{"error": customerrors.ErrInternal.Error()})
			c.Abort()
			return
		}

		c.Set(userCtx, claims.UserID)
		c.Next()
	}
}

func (h *Handler) RequireAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		h.logger.Debug("Calling RequireAuth")
		_, exists := c.Get(userCtx)
		if !exists {
			c.JSON(401, gin.H{"error": customerrors.ErrUserNotAuthenticated.Error()})
			c.Abort()
			return
		}
		c.Next()
	}
}

func (h *Handler) AccessPostAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		h.logger.Debug("Calling AccessPostAuth")
		if c.Request.Method == http.MethodPost {
			var req dto.RequestCreatePasta
			if err := c.ShouldBindJSON(&req); err != nil {
				c.JSON(400, gin.H{"error": customerrors.ErrInvalidRequst.Error()})
				c.Abort()
				return
			}
			c.Set(requestCtx, req)

			if strings.ToLower(req.Visibility) == visibilityPrivate {
				_, exists := c.Get(userCtx)
				if !exists {
					c.JSON(401, gin.H{"error": customerrors.ErrUserNotAuthenticated.Error()})
					c.Abort()
					return
				}
			}
			c.Next()
			return
		}
	}
}

func (h *Handler) AccessByKeyAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		h.logger.Debug("Calling AccessByKeyAuth")

		hash := c.Param("hash")
		ctx := c.Request.Context()

		visibility, err := h.servises.Pasta.GetVisibility(ctx, hash)
		if err != nil {
			if errors.Is(err, customerrors.ErrPastaNotFound) {
				c.JSON(404, gin.H{"error": customerrors.ErrPastaNotFound.Error()})
			} else {
				h.logger.Error("Internal error on GetVisibility", logging.ErrAttr(err))
				c.JSON(500, gin.H{"error": customerrors.ErrInternal.Error()})
			}
			c.Abort()
			return
		}
		c.Set(visibilityCtx, visibility)
		if c.Request.Method == http.MethodPut {
			userID, err := h.servises.Pasta.GetUserID(ctx, hash)
			if err != nil {
				h.logger.Error("Internal error on GetVisibility", logging.ErrAttr(err))
				c.JSON(500, gin.H{"error": customerrors.ErrInternal.Error()})
				c.Abort()
				return
			}
			if userID == 0 {
				c.JSON(401, gin.H{"error": customerrors.ErrUserNotAuthenticated.Error()})
				c.Abort()
				return
			}
			c.Next()
			return
		}

		if visibility == visibilityPrivate {
			_, exists := c.Get(userCtx)
			if !exists {
				c.JSON(401, gin.H{"error": customerrors.ErrUserNotAuthenticated.Error()})
				c.Abort()
				return
			}
		}
		c.Next()
	}
}

func (h *Handler) GetUserID(c *gin.Context) (int, error) {
	rawUserID, exists := c.Get(userCtx)
	if !exists {
		return 0, nil
	}

	userID, ok := rawUserID.(int)
	if !ok {
		h.logger.Error("Invalid type for userCtx")
		return 0, customerrors.ErrInternal
	}
	return userID, nil
}

func (h *Handler) GetRequest(c *gin.Context) (*dto.RequestCreatePasta, error) {
	rawRequest, exists := c.Get(requestCtx)
	if !exists {
		return nil, nil
	}

	request, ok := rawRequest.(dto.RequestCreatePasta)
	if !ok {
		h.logger.Error("Invalid type for requestCtx")
		return nil, customerrors.ErrInternal
	}
	return &request, nil
}

func (h *Handler) GetVisibility(c *gin.Context) (string, error) {
	rawVisibility, exists := c.Get(visibilityCtx)
	if !exists {
		return "", nil
	}

	visibility, ok := rawVisibility.(string)
	if !ok {
		h.logger.Error("invalid type for requestCtx")
		return "", customerrors.ErrInternal
	}
	return visibility, nil
}
