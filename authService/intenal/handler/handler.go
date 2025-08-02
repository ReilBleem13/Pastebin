package handler

import (
	domain "authService/intenal/domain/service"
	"authService/pkg/logging"

	"github.com/gin-gonic/gin"
)

type Handler struct {
	servise domain.Authorization
	logger  *logging.Logger
}

func NewHandler(servises domain.Authorization, logger *logging.Logger) *Handler {
	return &Handler{
		servise: servises,
		logger:  logger,
	}
}

func (h *Handler) InitRoutes() *gin.Engine {
	router := gin.New()

	router.GET("/health", func(c *gin.Context) {
		c.String(200, "OK")
	})

	auth := router.Group("/auth")
	{
		auth.POST("/sign-up", h.SignUp)
		auth.POST("/sign-in", h.SignIn)
		auth.POST("/refresh", h.RefreshTokenHandler)
	}
	return router
}
