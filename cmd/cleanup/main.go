package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"pastebin/internal/repository/minio"
	"pastebin/internal/repository/postgres"
	"pastebin/internal/service"
	"syscall"
	"time"

	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
	"github.com/spf13/viper"
)

func main() {
	if err := initConfig(); err != nil {
		log.Fatalf("error initializing configs: %s", err.Error())
	}

	if err := godotenv.Load(); err != nil {
		log.Fatalf("error loading env variables: %s", err.Error())
	}

	db, err := postgres.NewPostgresDB(postgres.Config{
		Host:     viper.GetString("db.host"),
		Port:     viper.GetString("db.port"),
		Username: viper.GetString("db.username"),
		DBName:   viper.GetString("db.dbname"),
		SSLMode:  viper.GetString("db.sslmode"),
		Password: os.Getenv("DB_PASSWORD"),
	})
	if err != nil {
		log.Fatalf("failed to initialize db: %s", err.Error())
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	repo := postgres.NewRepository(db.DB())
	minioClient := minio.NewMinioClient(minio.Config{
		MinioEndPoint:     viper.GetString("minio.endpoint"),
		BucketName:        viper.GetString("minio.bucket"),
		MinioRootUser:     viper.GetString("minio.rootuser"),
		MinioRootPassword: os.Getenv("MinioRootPassword"),
		MinioUseSSL:       viper.GetBool("minio.ssl"),
	}, 10)

	err = minioClient.InitMinio()
	if err != nil {
		log.Fatalf("failed to initialize minio: %s", err.Error())
	}

	cleanupService := service.NewExpiredDeleteService(repo.Minio, minioClient)

	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	go func() {
		for {
			select {
			case <-ticker.C:
				log.Println("Cleanup expired pasta...")
				if err := cleanupService.CleanupExpiredPasta(ctx); err != nil {
					log.Printf("Failed to cleanup expired pasta: %v", err)
				}
			case <-ctx.Done():
				log.Println("Cleanup ticker stopped")
				return
			}
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Cleanup is shutting down...")
	db.Close()
	minioClient.Close()
}

func initConfig() error {
	viper.AddConfigPath("configs")
	viper.SetConfigName("config")
	return viper.ReadInConfig()
}
