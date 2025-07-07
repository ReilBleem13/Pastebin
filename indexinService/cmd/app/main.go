package main

import (
	"context"
	elastic "indexing_service/elasticsearch"
	"indexing_service/handler"
	"indexing_service/kafka"
	s3 "indexing_service/minio"
	logging "indexing_service/utils/logger"
	"log"
)

func main() {
	logger := logging.GetLogger()

	minioClient, err := s3.NewMinioClient(context.TODO(), s3.Config{
		Host:     "localhost:9000",
		Bucket:   "chukki",
		RootUser: "root",
		Password: "minio_password",
		SSL:      false,
	}, 10)
	if err != nil {
		log.Fatal(err.Error())
	}

	elasticCLient, err := elastic.NewElasticClient([]string{"http://localhost:9200"})
	if err != nil {
		log.Fatal(err.Error())
	}

	handler := handler.NewHandler(minioClient, elasticCLient)

	c, err := kafka.NewConsumer(handler, logger, "localhost:9092", "text.indexed", "indexing", 10)
	if err != nil {
		log.Fatal(err.Error())
	}
	c.Start()
}
