package repository

import (
	"pastebin/internal/domain"
	"pastebin/internal/repository/cache"
	"pastebin/internal/repository/database"
	"pastebin/internal/repository/elasticsearch"
	"pastebin/internal/repository/s3"
	"pastebin/pkg/workerpool"

	elastic "github.com/elastic/go-elasticsearch/v8"
	"github.com/go-redis/redis/v8"
	"github.com/jmoiron/sqlx"
	"github.com/minio/minio-go/v7"
)

type Repository struct {
	Database domain.Database
	Cache    domain.Cache
	S3       domain.S3
	Elastic  domain.Elastic
}

func NewRepository(db *sqlx.DB, redis *redis.Client, minio *minio.Client,
	elc *elastic.Client, pool *workerpool.WorkerPool, bucket string) *Repository {
	return &Repository{
		Database: database.NewDatabase(db),
		Cache:    cache.NewCache(redis),
		S3:       s3.NewS3(minio, pool, bucket),
		Elastic:  elasticsearch.NewElastic(elc),
	}
}
