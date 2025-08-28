package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"pastebin/cmd/workers/scanner"
	"pastebin/internal/config"
	"pastebin/internal/handler"
	"pastebin/internal/infrastructure/elastic"
	"pastebin/internal/infrastructure/minio"
	"pastebin/internal/infrastructure/postgres"
	"pastebin/internal/infrastructure/redis"
	repostitory "pastebin/internal/repository"
	"pastebin/internal/service"
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

	logging.WithAttrs(ctxWithLogger,
		logging.StringAttr("username", cfg.Storage.Username),
		logging.StringAttr("password", cfg.Storage.Password),
		logging.StringAttr("host", cfg.Storage.Host),
		logging.StringAttr("port", cfg.Storage.Port),
		logging.StringAttr("database", cfg.Storage.Dbname),
	).Info("Postgres initializing")
	dbURL := fmt.Sprintf("host=%s port=%s user=%s dbname=%s password=%s sslmode=%s",
		cfg.Storage.Host, cfg.Storage.Port, cfg.Storage.Username, cfg.Storage.Dbname, cfg.Storage.Password, cfg.Storage.Sslmode)
	postgres, err := postgres.NewPostgresDB(ctx, dbURL)
	if err != nil {
		logger.Error("Failed to initialize postgres", logging.ErrAttr(err))
		return
	}
	defer postgres.Close()

	logging.WithAttrs(ctx,
		logging.StringAttr("addr", cfg.Minio.Addr),
		logging.StringAttr("username", cfg.Minio.User),
		logging.StringAttr("password", "<REMOVED>"),
		logging.StringAttr("bucket", cfg.Minio.Bucket),
		logging.IntAttr("maxRetries", cfg.Minio.MaxRetries),
		logging.BoolAttr("ssl-mode", cfg.Minio.Ssl),
	).Info("Minio initizializing")
	minio, err := minio.NewMinioClient(ctx, cfg.Minio, 10)
	if err != nil {
		logger.Error("Failed to initialize minio", logging.ErrAttr(err))
		return
	}
	defer minio.Close(ctxWithLogger)

	redis := redis.NewRedisClient(ctxWithLogger, cfg.Redis)
	if err := redis.Client.Ping(ctx).Err(); err != nil {
		logger.Error("Failed to initialize redis", logging.ErrAttr(err))
		return
	}
	logging.WithAttrs(ctx,
		logging.StringAttr("mode", cfg.Redis.Mode),
		logging.StringAttr("addr", strings.Join(cfg.Redis.Addrs, ", ")),
		logging.StringAttr("addrs", cfg.Redis.Addr),
		logging.StringAttr("password", "<REMOVED>"),
		logging.StringAttr("dialTimeout", cfg.Redis.DialTimeout.String()),
		logging.StringAttr("readTimeout", cfg.Redis.ReadTimeout.String()),
		logging.StringAttr("writeTimeout", cfg.Redis.WriteTimeout.String()),
		logging.IntAttr("poolSize", cfg.Redis.PoolSize),
		logging.IntAttr("maxRetries", cfg.Redis.MaxRetries),
	).Info("Redis initialized")
	defer redis.Client.Close()

	elastic, err := elastic.NewElasticClient(ctxWithLogger, cfg.Elastic)
	if err != nil {
		logger.Error("Failed to initialize elasticsearch", logging.ErrAttr(err))
		return
	}
	index, err := elastic.CreateSearchIndex(cfg.Elastic.Index)
	if err != nil {
		logger.Error("Failed to create elasticsearch index", logging.ErrAttr(err))
		return
	}
	logging.WithAttrs(ctx,
		logging.StringAttr("addresses", strings.Join(cfg.Elastic.Addrs, ", ")),
		logging.StringAttr("index", cfg.Elastic.Index),
	).Info("ElasticSearch initialized")
	defer elastic.Close()

	repo := repostitory.NewRepository(postgres.Client(), redis.Client,
		minio.Client(), elastic.Client(), minio.Pool(), cfg.Minio.Bucket, index)

	services := service.NewService(ctxWithLogger, repo)
	handlers := handler.NewHandler(ctxWithLogger, services)

	srv := new(handler.Server)
	go func() {
		if err := srv.Run(cfg.App.Port, handlers.InitRoutes(cfg.App.Mode)); err != nil {
			logger.Error("Failed run server", logging.ErrAttr(err))
			return
		}
	}()

	go scanner.StartScannerWorker(ctxWithLogger, services)

	logger.Info("Server is running")
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	logger.Info("Server is shutting down")

	if err := srv.Shutdown(ctx); err != nil {
		logger.Error("Failed to shutdown server", logging.ErrAttr(err))
		return
	}
}
