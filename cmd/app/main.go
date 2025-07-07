package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"pastebin/internal/config"
	"pastebin/internal/handler"
	"pastebin/internal/infrastructure/elastic"
	"pastebin/internal/infrastructure/kafka"
	"pastebin/internal/infrastructure/minio"
	"pastebin/internal/infrastructure/postgres"
	"pastebin/internal/infrastructure/redis"
	repostitory "pastebin/internal/repository"
	"pastebin/internal/service"
	"pastebin/pkg/logging"
	"syscall"

	_ "github.com/lib/pq"
)

const (
	prodConfig string = "config.yml"
)

func main() {
	logger := logging.GetLogger()
	cfg := config.GetConfig(prodConfig)

	// инициализация postgres
	dbURL := fmt.Sprintf("host=%s port=%s user=%s dbname=%s password=%s sslmode=%s",
		cfg.Storage.Host, cfg.Storage.Port, cfg.Storage.Username, cfg.Storage.Dbname, cfg.Storage.Password, cfg.Storage.Sslmode)
	postgres, err := postgres.NewPostgresDB(context.TODO(), dbURL)
	if err != nil {
		logger.Fatalf("failed to initialize postgres: %v", err)
	}

	// инициализация minio
	minio, err := minio.NewMinioClient(context.TODO(), cfg.Minio, 10)
	if err != nil {
		logger.Fatalf("failed to initialize minio: %v", err)
	}

	// инициализация redis
	redis, err := redis.NewRedisClient(context.TODO(), cfg.Redis)
	if err != nil {
		logger.Fatalf("failed to initialize redis: %v", err)
	}

	// инициализация elastic
	elastic, err := elastic.NewElasticClient(cfg.Elastic)
	if err != nil {
		logger.Fatalf("failed to initialize elasticsearch: %v", err)
	}
	if err := elastic.EnsureIndex(cfg.Elastic.Index); err != nil {
		logger.Fatalf("failed to create elasticsearch index: %v", err)
	}

	producer, err := kafka.NewProducer(cfg.Kafka.Address)
	if err != nil {
		logger.Fatalf("failed to create producer: %v", err)
	}

	// инициализация repository
	repo := repostitory.NewRepository(postgres.Client(), redis.Client(),
		minio.Client(), elastic.Client(), minio.Pool(), cfg.Minio.Bucket)

	services := service.NewService(repo, producer, logger)
	handlers := handler.NewHandler(services, logger)

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
	minio.Close(context.TODO())
	producer.Close()

	if err := srv.Shutdown(context.Background()); err != nil {
		log.Fatalf("error occured on server shutting down: %s", err.Error())
	}
}
