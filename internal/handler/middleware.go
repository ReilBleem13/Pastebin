package handler

import (
	"errors"
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
		if len(header) == 0 {
			c.JSON(401, gin.H{
				"error": "token is empty",
			})
			c.Abort()
			return
		}

		if !strings.HasPrefix(header, "Bearer ") {
			c.JSON(401, gin.H{
				"error": "invalid token prefix",
			})
			c.Abort()
			return
		}

		token := strings.TrimPrefix(header, "Bearer ")

		claims, err := utils.VerifyAccessToken(token)
		if err != nil {
			c.JSON(401, gin.H{
				"error": "failed to verify access token",
			})
			c.Abort()
			return
		}
		c.Set(userCtx, claims.UserID)
		c.Next()
	}
}

func (h *Handler) AccessMiddleWare() gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.Request.Method == "POST" {
			var request dto.RequestCreatePasta
			if err := c.BindJSON(&request); err != nil {
				c.JSON(400, gin.H{"error": err})
				c.Abort()
				return
			}
			c.Set("request", request)

			if request.Visibility != nil {
				if *request.Visibility == "private" {
					h.AuthMiddleWare()(c)
					if c.IsAborted() {
						return
					}
				}
			}
			c.Next()
			return
		}

		hash := c.Param("objectID")
		visibility, err := h.servises.DBMinio.GetVisibility(hash)
		if err != nil {
			c.JSON(400, gin.H{"error": err})
			c.Abort()
			return
		}
		c.Set(visibilityCtx, visibility)

		if visibility == "private" {
			h.AuthMiddleWare()(c)
			if c.IsAborted() {
				return
			}
		}
		c.Next()
	}
}

func ptrSrt(s string) *string {
	return &s
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

func (h *Handler) GetRequest(c *gin.Context) (dto.RequestCreatePasta, error) {
	request, exists := c.Get(requestCtx)
	if !exists {
		return dto.RequestCreatePasta{}, errors.New("user id not found")
	}
	requestNew, ok := request.(dto.RequestCreatePasta)
	if !ok {
		return dto.RequestCreatePasta{}, errors.New("user id is of invalid type")
	}

	return requestNew, nil
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
