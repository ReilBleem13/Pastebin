package handler

import (
	"pastebin/internal/service"
	"pastebin/pkg/logging"

	"github.com/gin-gonic/gin"
)

type Handler struct {
	servises *service.Service
	logger   *logging.Logger
}

func NewHandler(servises *service.Service, logger *logging.Logger) *Handler {
	return &Handler{
		servises: servises,
		logger:   logger,
	}
}

func (h *Handler) InitRoutes() *gin.Engine {
	router := gin.New()

	auth := router.Group("/")
	auth.Use(h.AuthMiddleWare())
	{
		auth.POST("/create", h.AccessPostMiddleware(), h.CreatePastaHandler) // обработк ошибки при пустом json

		auth.GET("/receive/:objectID", h.AccessByKeyMiddleware(), h.GetPastaHandler)
		auth.GET("/v1/paginate", h.PaginatePublicHandler)
		// auth.GET("/v1/paginate/me", h.RequireAuth(), h.PaginateUserIdHandler)

		auth.DELETE("/delete/:objectID", h.AccessByKeyMiddleware(), h.DeletePastaHandler)
	}

	start := router.Group("/auth")
	{
		start.POST("/sign-up", h.SignUp)
		start.POST("/sign-in", h.SignIn)
	}

	return router
}
