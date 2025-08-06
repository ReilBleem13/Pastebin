package handler

import (
	elastic "cleanService/infrastucture/elasticsearch"
	"cleanService/infrastucture/kafka"
	s3 "cleanService/infrastucture/minio"
	"cleanService/infrastucture/postgres"
	"cleanService/utils/retry"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/theartofdevel/logging"
)

type Handler struct {
	s3      s3.Minio
	elastic elastic.Elastic
	db      postgres.Postgres
	logger  *logging.Logger
}

func NewHandler(ctx context.Context, s3Client *s3.MinioClient, elasticClient *elastic.ElasticClient, postgresClient *postgres.PostgresClient) *Handler {
	return &Handler{
		s3:      s3Client,
		elastic: elasticClient,
		db:      postgresClient,
		logger:  logging.L(ctx),
	}
}

func (h *Handler) DeleteExpiredPastas(rawMessage []byte) error {
	var message kafka.CleanExpired
	if err := json.Unmarshal(rawMessage, &message); err != nil {
		return err
	}

	h.logger.Info(fmt.Sprintf("Consumer got %d pastas for delete", len(message.ObjectIDs)))

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	wg := &sync.WaitGroup{}
	errCh := make(chan error, 3)

	wg.Add(1)
	go func() {
		defer wg.Done()

		err := retry.WithRetry(ctx, func() error {
			err := h.db.DeletePastas(ctx, message.ObjectIDs)
			return err
		}, retry.IsRetryableErrorDatabase)
		if err != nil {
			errCh <- fmt.Errorf("failed to delete from database: %w", err)
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()

		err := retry.WithRetry(ctx, func() error {
			err := h.s3.Delete(ctx, message.ObjectIDs)
			return err
		}, retry.IsRetryableErrorMinio)
		if err != nil {
			errCh <- fmt.Errorf("failed to delete from s3: %w", err)
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()

		err := retry.WithRetry(ctx, func() error {
			err := h.elastic.DeleteDocuments(ctx, message.ObjectIDs, message.Index)
			return err
		}, retry.IsRetryableErrorElastic)
		if err != nil {
			errCh <- fmt.Errorf("failed to delete from elastic: %w", err)
		}
	}()
	go func() {
		wg.Wait()
		close(errCh)
	}()

	var errs []error
	for err := range errCh {
		errs = append(errs, err)
	}

	if len(errs) > 0 {
		return fmt.Errorf("some deletions failed: %w", errors.Join(errs...))
	}

	h.logger.Info(fmt.Sprintf("Amount of %d pastas was deleted", len(message.ObjectIDs)))
	return nil
}

func (h *Handler) InitRoutes(ginMode string) *gin.Engine {
	gin.SetMode(ginMode)
	router := gin.New()

	router.Use(gin.Recovery(), h.Logger())

	router.GET("/health", func(c *gin.Context) {
		c.String(200, "OK")
	})
	return router
}

func (h *Handler) Logger() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()

		c.Next()

		latency := time.Since(start)
		status := c.Writer.Status()
		method := c.Request.Method
		path := c.Request.URL.Path
		clientIP := c.ClientIP()

		h.logger.Info("Request",
			logging.StringAttr("[method]", method),
			logging.StringAttr("[path]", path),
			logging.StringAttr("[status]", strconv.Itoa(status)),
			logging.StringAttr("[latency]", latency.String()),
			logging.StringAttr("[clientIP]", clientIP),
		)
	}
}
