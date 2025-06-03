package handler

import (
	"pastebin/pkg/service"

	"github.com/gin-gonic/gin"
)

type Handler struct {
	servises service.Service
}

func NewHandler(servises service.Service) *Handler {
	return &Handler{
		servises: servises,
	}
}

func (h *Handler) InitRoutes() *gin.Engine {
	router := gin.New()

	minioRoutes := router.Group("/files")

	{
		minioRoutes.POST("/", h.CreateOne)
		minioRoutes.GET("/:objectID", h.GetOne)
	}
	return router
}
