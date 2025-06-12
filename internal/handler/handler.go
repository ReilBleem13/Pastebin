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

	auth := router.Group("/")
	auth.Use(h.AuthMiddleWare())
	{
		auth.POST("/create", h.AccessPostMiddleware(), h.CreatePastaHandler) // обработк ошибки при пустом json

		auth.GET("/receive/:objectID", h.AccessByKeyMiddleware(), h.GetPastaHandler)
		auth.GET("/out", h.PaginatePublicHandler)
		auth.GET("/out/me", h.RequireAuth(), h.PaginateUserIdHandler)

		auth.DELETE("/delete/:objectID", h.AccessByKeyMiddleware(), h.DeletePastaHandler)
	}

	router.GET("/test", h.PaginatePublicHandler)

	start := router.Group("/auth")
	{
		start.POST("/sign-up", h.SignUp)
		start.POST("/sign-in", h.SignIn)
	}

	return router
}
