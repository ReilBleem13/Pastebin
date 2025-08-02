package handler

import (
	"context"
	"pastebin/internal/service"

	"github.com/gin-gonic/gin"
	"github.com/theartofdevel/logging"
)

type Handler struct {
	servises *service.Service
	logger   *logging.Logger
}

func NewHandler(ctx context.Context, servises *service.Service) *Handler {
	return &Handler{
		servises: servises,
		logger:   logging.L(ctx),
	}
}

func (h *Handler) InitRoutes() *gin.Engine {
	gin.SetMode(gin.ReleaseMode)
	router := gin.New()

	auth := router.Group("/")
	auth.Use(h.AuthMiddleWare())
	{
		auth.GET("/receive/:hash", h.AccessByKeyAuth(), h.GetPastaHandler)
		auth.POST("/create", h.AccessPostAuth(), h.CreatePastaHandler) // обработк ошибки при пустом json
		auth.PUT("/update/:hash", h.RequireAuth(), h.AccessByKeyAuth(), h.UpdateHandler)
		auth.DELETE("/delete/:hash", h.AccessByKeyAuth(), h.DeletePastaHandler)

		auth.GET("/paginate", h.PaginatePublicHandler)
		auth.GET("/paginate/me", h.RequireAuth(), h.PaginateForUserHandler)
		auth.GET("/search", h.SearchHandler)

		auth.POST("/favorite/create/:hash", h.RequireAuth(), h.AccessByKeyAuth(), h.CreateFavorite)
		auth.GET("/favorite/:favorite_id", h.RequireAuth(), h.GetFavorite)
		auth.DELETE("/favorite/:favorite_id", h.RequireAuth(), h.DeleteFavorite)
		auth.GET("/favorite/paginate", h.RequireAuth(), h.PaginateFavorites)
	}
	return router
}
