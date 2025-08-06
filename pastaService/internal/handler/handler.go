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

func (h *Handler) InitRoutes(ginMode string) *gin.Engine {
	gin.SetMode(ginMode)
	router := gin.New()

	router.Use(gin.Recovery(), h.Logger())

	router.GET("/health", func(c *gin.Context) {
		c.String(200, "OK")
	})

	api := router.Group("/")
	api.Use(h.AuthMiddleWare())
	{
		api.POST("/create", h.AccessCreate(), h.CreatePastaHandler)
		api.GET("/receive/:hash", h.AccessHash(), h.GetPastaHandler)
		api.DELETE("/delete/:hash", h.AccessHash(), h.DeletePastaHandler)

		api.GET("/paginate", h.PaginatePublicHandler)
		api.GET("/search", h.SearchHandler)

		private := api.Group("/")
		private.Use(h.RequireAuth())
		{
			private.PUT("/update/:hash", h.UpdateHandler)
			private.GET("/paginate/me", h.PaginateForUserHandler)

			favorites := private.Group("/favorite")
			{
				favorites.GET("/:favorite_id", h.GetFavorite)
				favorites.DELETE("/:favorite_id", h.DeleteFavorite)
				favorites.POST("/create/:hash", h.CreateFavorite)
				favorites.GET("/paginate", h.PaginateFavorites)
			}
		}
	}
	return router
}
