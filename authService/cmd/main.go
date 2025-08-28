package main

import (
	"authService/intenal/config"
	"authService/intenal/handler"
	"authService/intenal/infrastructure/postgres"
	"authService/intenal/repository"
	"authService/intenal/service"
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

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

	repo := repository.NewAuthRepository(postgres.Client())
	srv := service.NewScannerService(ctxWithLogger, repo)
	handlers := handler.NewHandler(ctxWithLogger, srv)

	server := new(handler.Server)
	go func() {
		if err := server.Run(cfg.App.Port, handlers.InitRoutes(cfg.App.Mode)); err != nil {
			logger.Error("Failed run server", logging.ErrAttr(err))
			return
		}
	}()

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
