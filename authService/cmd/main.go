package main

import (
	"authService/intenal/config"
	"authService/intenal/handler"
	"authService/intenal/infrastructure/postgres"
	"authService/intenal/repository"
	"authService/intenal/service"
	"authService/pkg/logging"
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
)

const (
	prodConfig string = "config.yml"
)

func main() {
	logger := logging.GetLogger()
	cfg := config.GetConfig(prodConfig, logger)

	dbURL := fmt.Sprintf("host=%s port=%s user=%s dbname=%s password=%s sslmode=%s",
		cfg.Storage.Host, cfg.Storage.Port, cfg.Storage.Username, cfg.Storage.Dbname, cfg.Storage.Password, cfg.Storage.Sslmode)
	logger.Infof("Результат: %s", dbURL)
	postgres, err := postgres.NewPostgresDB(context.TODO(), dbURL)
	if err != nil {
		logger.Fatalf("failed to initialize postgres: %v", err)
	}

	repo := repository.NewAuthRepository(postgres.Client())
	srv := service.NewScannerService(repo, logger)
	handlers := handler.NewHandler(srv, logger)

	server := new(handler.Server)
	go func() {
		if err := server.Run(cfg.Listen.Port, handlers.InitRoutes()); err != nil {
			log.Fatalf("status: %s", err.Error())
		}
	}()

	logger.Info("Server is running...")
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	logger.Info("Server is shutting down...")

	postgres.Close()

	if err := server.Shutdown(context.Background()); err != nil {
		log.Fatalf("status: %v", err.Error())
	}
}
