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

	mainRouter := router.Group("/files")
	mainRouter.Use(h.AccessMiddleWare())
	{
		mainRouter.POST("/", h.CreatePastaHandler)
		mainRouter.GET("/:objectID", h.GetPastaHandler)
		mainRouter.DELETE("/:objectID", h.DeletePastaHandler)
	}

	signUpIn := router.Group("/auth")
	{
		signUpIn.POST("/sign-up", h.SignUp)
		signUpIn.POST("/sign-in", h.SignIn)
	}

	router.GET("/test", h.PaginatePastaHandler)
	return router
}
