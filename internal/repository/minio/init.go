package minio

import (
	"context"
	"pastebin/internal/models"

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

type FileRepository interface {
	InitMinio() error

	StoreFile(ctx context.Context, owner string, data []byte, isPassword map[string]string) (models.Paste, error)
	GetFile(ctx context.Context, objectID string) (string, error)
	GetFiles(ctx context.Context, objectIDs []string) ([]string, error)
	DeleteFile(ctx context.Context, objectID string) error
	DeleteFiles(ctx context.Context, objectIDs []string) error

	PaginateFiles(ctx context.Context, maxKeys int, startAfter, prefix string) ([]string, string, error)
	PaginateFilesByUserID(ctx context.Context, maxKeys int, startAfter, prefix string) ([]string, string, error)
}

type minioClient struct {
	mc  *minio.Client
	cfg Config
}

func NewMinioClient(cfg Config) FileRepository {
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
