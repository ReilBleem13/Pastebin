package handler

import (
	"pastebin/internal/service"

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
	minioRoutes.Use(h.AccessMiddleWare())
	{
		minioRoutes.POST("/", h.CreatePastaHandler)
		minioRoutes.GET("/:objectID", h.GetPastaHandler)
		// minioRoutes.GET("/raw/:objectID", h.GetRawPastaHandler)
	}

	signUpIn := router.Group("/auth")
	{
		signUpIn.POST("/sign-up", h.SignUp)
		signUpIn.POST("/sign-in", h.SignIn)
	}

	return router
}
