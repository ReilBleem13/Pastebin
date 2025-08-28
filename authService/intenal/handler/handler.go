package handler

import (
	domain "authService/intenal/domain/service"
	"context"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/theartofdevel/logging"
)

type Handler struct {
	servise domain.Authorization
	logger  *logging.Logger
}

func NewHandler(ctx context.Context, servises domain.Authorization) *Handler {
	return &Handler{
		servise: servises,
		logger:  logging.L(ctx),
	}
}

func (h *Handler) InitRoutes(ginMode string) *gin.Engine {
	gin.SetMode(ginMode)
	router := gin.New()

	router.Use(gin.Recovery(), h.Logger())

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
