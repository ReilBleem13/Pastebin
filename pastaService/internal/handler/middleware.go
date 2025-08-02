package handler

import (
	"errors"
	customerrors "pastebin/internal/errors"
	"pastebin/internal/models"
	"pastebin/internal/utils"
	"pastebin/pkg/dto"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/theartofdevel/logging"
)

const (
	authorizationHeader = "Authorization"
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
		if !h.requireUserAuth(c) {
			return
		}
		c.Next()
	}
}

func (h *Handler) AccessCreate() gin.HandlerFunc {
	return func(c *gin.Context) {
		h.logger.Debug("Calling AccessPostAuth")
		var req dto.RequestCreatePasta
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(400, gin.H{"error": customerrors.ErrInvalidRequst.Error()})
			c.Abort()
			return
		}
		c.Set(requestCtx, req)

		if req.Visibility == string(models.VisibilityPrivate) {
			if !h.requireUserAuth(c) {
				return
			}
		}
		c.Next()
	}
}

func (h *Handler) AccessHash() gin.HandlerFunc {
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

		if visibility == string(models.VisibilityPrivate) {
			if !h.requireUserAuth(c) {
				return
			}
		}
		c.Next()
	}
}

func (h *Handler) Logger() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()

		c.Next()

		latency := time.Since(start)
		status := c.Writer.Status()
		method := c.Request.Method
		path := c.Request.URL.Path
		clientIP := c.ClientIP()

		h.logger.Info("Request",
			logging.StringAttr("[method]", method),
			logging.StringAttr("[path]", path),
			logging.StringAttr("[status]", strconv.Itoa(status)),
			logging.StringAttr("[latency]", latency.String()),
			logging.StringAttr("[clientIP]", clientIP),
		)
	}
}

func (h *Handler) requireUserAuth(c *gin.Context) bool {
	_, exists := c.Get(userCtx)
	if !exists {
		c.JSON(401, gin.H{"error": customerrors.ErrUserNotAuthenticated.Error()})
		c.Abort()
		return false
	}
	return true
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
