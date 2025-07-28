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
		auth.GET("/receive/:hash", h.AccessByKeyMiddleware(), h.GetPastaHandler)
		auth.POST("/create", h.AccessPostMiddleware(), h.CreatePastaHandler) // обработк ошибки при пустом json
		auth.PUT("/update/:hash", h.RequireAuth(), h.AccessByKeyMiddleware(), h.UpdateHandler)
		auth.DELETE("/delete/:hash", h.AccessByKeyMiddleware(), h.DeletePastaHandler)

		auth.GET("/paginate", h.PaginatePublicHandler)
		auth.GET("/paginate/me", h.RequireAuth(), h.PaginateForUserHandler)
		auth.GET("/search", h.SearchHandler)

		auth.POST("/favorite/create/:hash", h.RequireAuth(), h.AccessByKeyMiddleware(), h.CreateFavorite)
		auth.GET("/favorite/:favorite_id", h.RequireAuth(), h.GetFavorite)
		auth.DELETE("/favorite/:favorite_id", h.RequireAuth(), h.DeleteFavorite)
		auth.GET("/favorite/paginate", h.RequireAuth(), h.PaginateFavorites)
	}
	return router
}
