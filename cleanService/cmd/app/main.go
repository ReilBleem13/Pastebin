package main

import (
	"cleanService/config"
	"cleanService/handler"
	elastic "cleanService/infrastucture/elasticsearch"
	"cleanService/infrastucture/kafka"
	s3 "cleanService/infrastucture/minio"
	"cleanService/infrastucture/postgres"
	logging "cleanService/utils/logger"
	"context"
	"fmt"

	_ "github.com/lib/pq"
)

const (
	prodConfig string = "config.yml"
)

func main() {
	logger := logging.GetLogger()
	cfg := config.GetConfig(prodConfig, logger)

	minioClient, err := s3.NewMinioClient(context.TODO(), cfg.Minio, 10)
	if err != nil {
		logger.Fatalf("failed to initialize minio: %v", err)
	}

	elasticCLient, err := elastic.NewElasticClient(cfg.Elastic.Addresses)
	if err != nil {
		logger.Fatalf("failed to initialize elasticsearch: %v", err)
	}

	dbURL := fmt.Sprintf("host=%s port=%s user=%s dbname=%s password=%s sslmode=%s",
		cfg.Storage.Host, cfg.Storage.Port, cfg.Storage.Username, cfg.Storage.Dbname, cfg.Storage.Password, cfg.Storage.Sslmode)
	postgresClient, err := postgres.NewPostgresClient(context.TODO(), dbURL)
	if err != nil {
		logger.Fatalf("failed to initialize postgres: %v", err)
	}

	handler := handler.NewHandler(minioClient, elasticCLient, postgresClient, logger)

	c, err := kafka.NewConsumer(handler, logger, cfg.Kafka, 10)
	if err != nil {
		logger.Fatalf("failed to initialize new consumer: %v", err)
	}
	logger.Info("Consumer was created")
	c.Start()
}
