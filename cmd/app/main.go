package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"pastebin/internal/config"
	"pastebin/internal/handler"
	repostitory "pastebin/internal/repository"
	"pastebin/internal/repository/cache"
	"pastebin/internal/repository/database"
	"pastebin/internal/repository/elasticsearch"
	"pastebin/internal/repository/s3"
	"pastebin/internal/service"
	"pastebin/pkg/logging"
	"syscall"

	_ "github.com/lib/pq"
)

func main() {
	logger := logging.GetLogger()
	cfg := config.GetConfig()

	ctx := context.Background()

	// инициализация postgres
	postgres, err := database.NewPostgresDB(ctx, cfg.Storage)
	if err != nil {
		logger.Fatalf("failed to initialize postgres: %v", err)
	}

	// инициализация minio
	minio, err := s3.NewMinioClient(ctx, cfg.Minio, 10)
	if err != nil {
		logger.Fatalf("failed to initialize minio: %v", err)
	}

	// инициализация redis
	redis, err := cache.NewRedisClient(ctx, cfg.Redis)
	if err != nil {
		logger.Fatalf("failed to initialize redis: %v", err)
	}

	// инициализация elastic
	elastic, err := elasticsearch.NewElasticClient(cfg.Elastic)
	if err != nil {
		logger.Fatalf("failed to initialize elasticsearch: %v", err)
	}
	if err := elastic.EnsureIndex(cfg.Elastic.Index); err != nil {
		logger.Fatalf("failed to create elasticsearch index: %v", err)
	}

	// инициализация repository
	repo := repostitory.NewRepository(postgres.Client(), redis.Client(),
		minio.Client(), elastic.Client(), minio.Pool(), cfg.Minio.Bucket)

	services := service.NewService(repo)
	handlers := handler.NewHandler(services)

	srv := new(handler.Server)
	go func() {
		if err := srv.Run(cfg.Listen.Port, handlers.InitRoutes()); err != nil {
			log.Fatalf("error occured while running http server; %s", err.Error())
		}
	}()

	logger.Info("Server is running...")

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("Server is shutting down...")

	// зактрытие инфраструктуры
	postgres.Close()
	redis.Close()
	minio.Close(ctx)

	if err := srv.Shutdown(context.Background()); err != nil {
		log.Fatalf("error occured on server shutting down: %s", err.Error())
	}
}
