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

		auth.GET("/receive/:hash", h.AccessByKeyMiddleware(), h.GetPastaHandler)
		auth.GET("/v1/paginate", h.PaginatePublicHandler)
		auth.GET("/v1/paginate/me", h.RequireAuth(), h.PaginateForUserHandler)
		auth.GET("/search", h.SearchHandler)

		auth.PUT("/update/:hash", h.RequireAuth(), h.AccessByKeyMiddleware(), h.UpdateHandler)
		auth.DELETE("/delete/:hash", h.AccessByKeyMiddleware(), h.DeletePastaHandler)
	}
	return router
}
