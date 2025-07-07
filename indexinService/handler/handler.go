package handler

import (
	"context"
	"encoding/json"
	"fmt"
	elastic "indexing_service/elasticsearch"
	s3 "indexing_service/minio"
	"indexing_service/utils/retry"
	"log"
	"time"

	mykafka "indexing_service/kafka"
)

type Handler struct {
	s3      s3.Minio
	elastic elastic.Elastic
}

func NewHandler(s3Client *s3.MinioClient, elasticClient *elastic.ElasticClient) *Handler {
	return &Handler{
		s3:      s3Client,
		elastic: elasticClient,
	}
}

func (h *Handler) NewIndex(message []byte) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	log.Printf("Пришли данные: %s", string(message))

	var textIndexed mykafka.TextIndexed
	if err := json.Unmarshal(message, &textIndexed); err != nil {
		return fmt.Errorf("failed to unmarshal: %w", err)
	}

	var text []byte
	err := retry.WithRetry(ctx, func() error {
		var err error
		text, err = h.s3.Get(ctx, textIndexed.ObjectID)
		return err
	}, retry.IsRetryableErrorMinio)
	if err != nil {
		return fmt.Errorf("failed to get text from minio: %w", err)
	}

	if err := h.elastic.NewIndex(text, textIndexed.ObjectID, textIndexed.Index); err != nil {
		return fmt.Errorf("failed to index text: %w", err)
	}
	return nil
}
