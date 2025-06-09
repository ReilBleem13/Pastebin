package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"pastebin/internal/handler"
	"pastebin/internal/repository/database"
	"pastebin/internal/repository/minio"
	"pastebin/internal/repository/redis"
	"pastebin/internal/service"
	"syscall"

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

	db, err := database.NewPostgresDB(database.Config{
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
	minioClient := minio.NewMinioClient(minio.Config{
		MinioEndPoint:     viper.GetString("minio.endpoint"),
		BucketName:        viper.GetString("minio.bucket"),
		MinioRootUser:     viper.GetString("minio.rootuser"),
		MinioRootPassword: os.Getenv("MinioRootPassword"),
		MinioUseSSL:       viper.GetBool("minio.ssl"),
	})

	err = minioClient.InitMinio()
	if err != nil {
		log.Fatalf("failed to initialize minio: %s", err.Error())
	}

	redis := redis.NewRedisClient()
	err = redis.InitRedis()
	if err != nil {
		log.Fatalf("failed to initialize redis: %s", err.Error())
	}

	repos := database.NewRepository(db)
	services := service.NewService(repos, minioClient, redis)
	handlers := handler.NewHandler(*services)

	srv := new(handler.Server)
	go func() {
		if err := srv.Run(viper.GetString("port"), handlers.InitRoutes()); err != nil {
			log.Fatalf("error occured while running http server; %s", err.Error())
		}
	}()
	log.Println("Server is running...")

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Server is shutting down...")

	if err := srv.Shutdown(context.Background()); err != nil {
		log.Fatalf("error occured on server shitting down: %s", err.Error())
	}

	if err := db.Close(); err != nil {
		log.Fatalf("error occured on db connection close: %s", err.Error())
	}
}

func initConfig() error {
	viper.AddConfigPath("configs")
	viper.SetConfigName("config")
	return viper.ReadInConfig()
}
