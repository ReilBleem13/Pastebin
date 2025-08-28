package repository

import (
	domain "pastebin/internal/domain/repository"
	"pastebin/internal/repository/cache"
	"pastebin/internal/repository/database"
	"pastebin/internal/repository/elasticsearch"
	"pastebin/internal/repository/s3"
	"pastebin/pkg/workerpool"

	elastic "github.com/elastic/go-elasticsearch/v8"
	"github.com/jmoiron/sqlx"
	"github.com/minio/minio-go/v7"
	"github.com/redis/go-redis/v9"
)

type Repository struct {
	Database domain.Database
	Cache    domain.Cache
	S3       domain.S3
	Elastic  domain.Elastic
}

func NewRepository(db *sqlx.DB, redis redis.UniversalClient, minio *minio.Client,
	elc *elastic.Client, pool *workerpool.WorkerPool, bucket string, index string) *Repository {
	return &Repository{
		Database: database.NewDatabase(db),
		Cache:    cache.NewCache(redis),
		S3:       s3.NewS3(minio, pool, bucket),
		Elastic:  elasticsearch.NewElastic(elc, index),
	}
}
