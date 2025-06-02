package minio

import (
	"context"
	"pastebin/pkg/helpers"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

type Config struct {
	MinioEndPoint     string
	BucketName        string
	MinioRootUser     string
	MinioRootPassword string
	MinioUseSSL       bool
}

var AppConfig *Config

type Client interface {
	InitMinio() error
	CreateOne(file helpers.FileDataType) (string, error)
	CreateMany(map[string]helpers.FileDataType) ([]string, error)
	GetOne(objectID string) (string, error)
	GetMany(objectIDs []string) ([]string, error)
	DeleteOne(objectID string) error
	DeleteMany(objectIDs []string) error
}

type minioClient struct {
	mc  *minio.Client
	cfg Config
}

func NewMinioClient(cfg Config) Client {
	return &minioClient{
		cfg: cfg,
	}
}

func (m *minioClient) InitMinio() error {
	ctx := context.Background()
	client, err := minio.New(
		m.cfg.MinioEndPoint,
		&minio.Options{
			Creds: credentials.NewStaticV4(
				m.cfg.MinioRootUser,
				m.cfg.MinioRootPassword,
				"",
			),
			Secure: m.cfg.MinioUseSSL,
		},
	)
	if err != nil {
		return err
	}

	m.mc = client
	exists, err := m.mc.BucketExists(ctx, m.cfg.BucketName)
	if err != nil {
		return err
	}

	if !exists {
		err := m.mc.MakeBucket(ctx, m.cfg.BucketName, minio.MakeBucketOptions{})
		if err != nil {
			return err
		}
	}
	return nil
}
