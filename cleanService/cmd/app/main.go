package main

import (
	"cleanService/config"
	"cleanService/handler"
	elastic "cleanService/infrastucture/elasticsearch"
	"cleanService/infrastucture/kafka"
	s3 "cleanService/infrastucture/minio"
	"cleanService/infrastucture/postgres"
	"context"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"

	_ "github.com/lib/pq"
	"github.com/theartofdevel/logging"
)

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	cfg := config.GetConfig()

	level := "info"
	if cfg.App.Mode == "debug" {
		level = "debug"
	}
	logger := logging.NewLogger(
		logging.WithIsJSON(level != "debug"),
		logging.WithAddSource(level != "debug"),
		logging.WithLevel(level),
	)
	logger.With("mode", cfg.App.Mode).Info("App starting...")

	ctxWithLogger := logging.ContextWithLogger(ctx, logger)

	logging.WithAttrs(ctx,
		logging.StringAttr("addr", cfg.Minio.Addr),
		logging.StringAttr("username", cfg.Minio.User),
		logging.StringAttr("password", "<REMOVED>"),
		logging.StringAttr("bucket", cfg.Minio.Bucket),
		logging.IntAttr("maxRetries", cfg.Minio.MaxRetries),
		logging.BoolAttr("ssl-mode", cfg.Minio.Ssl),
	).Info("Minio initizializing")
	minioClient, err := s3.NewMinioClient(ctx, cfg.Minio, 10)
	if err != nil {
		logger.Error("Failed to initialize minio", logging.ErrAttr(err))
		return
	}
	defer minioClient.Close(ctx)

	logging.WithAttrs(ctx,
		logging.StringAttr("addr", strings.Join(cfg.Elastic.Addrs, ", ")),
		logging.StringAttr("index", cfg.Elastic.Index),
	).Info("ElasticSearch initialized")
	elasticCLient, err := elastic.NewElasticClient(ctxWithLogger, cfg.Elastic)
	if err != nil {
		logger.Error("Failed to initialize elasticsearch", logging.ErrAttr(err))
		return
	}
	defer elasticCLient.Close()

	logging.WithAttrs(ctxWithLogger,
		logging.StringAttr("username", cfg.Storage.Username),
		logging.StringAttr("password", cfg.Storage.Password),
		logging.StringAttr("host", cfg.Storage.Host),
		logging.StringAttr("port", cfg.Storage.Port),
		logging.StringAttr("database", cfg.Storage.Dbname),
	).Info("Postgres initializing")
	dbURL := fmt.Sprintf("host=%s port=%s user=%s dbname=%s password=%s sslmode=%s",
		cfg.Storage.Host, cfg.Storage.Port, cfg.Storage.Username, cfg.Storage.Dbname, cfg.Storage.Password, cfg.Storage.Sslmode)
	postgresClient, err := postgres.NewPostgresClient(context.TODO(), dbURL)
	if err != nil {
		logger.Error("Failed to initialize postgres", logging.ErrAttr(err))
		return
	}
	defer postgresClient.Close()

	hdl := handler.NewHandler(ctxWithLogger, minioClient, elasticCLient, postgresClient)

	server := new(handler.Server)
	go func() {
		if err := server.Run(cfg.App.Port, hdl.InitRoutes(cfg.App.Mode)); err != nil {
			logger.Error("Failed run server", logging.ErrAttr(err))
			return
		}
	}()

	c, err := kafka.NewConsumer(ctxWithLogger, hdl, cfg.Kafka, 10)
	if err != nil {
		logger.Error("Failed to initialize consumer", logging.ErrAttr(err))
		return
	}
	logger.Info("Consumer was created")

	go c.Start()
	defer c.Stop()

	logger.Info("Server is running...")
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	logger.Info("Server is shutting down...")

	if err := server.Shutdown(ctx); err != nil {
		logger.Error("Failed to shutdown server", logging.ErrAttr(err))
		return
	}
}
