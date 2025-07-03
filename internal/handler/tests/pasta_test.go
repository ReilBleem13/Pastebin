package tests

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"pastebin/internal/config"
	"pastebin/internal/handler"
	"pastebin/internal/repository"
	"pastebin/internal/repository/cache"
	"pastebin/internal/repository/database"
	elastic "pastebin/internal/repository/elasticsearch"
	"pastebin/internal/repository/s3"
	"pastebin/internal/service"
	"pastebin/pkg/dto"
	"pastebin/pkg/logging"
	"strings"
	"testing"
	"time"

	"github.com/elastic/go-elasticsearch/v8"
	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis/v8"
	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
	"github.com/minio/minio-go/v7"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

type TestEnv struct {
	PostgresURI   string
	MigrateURI    string
	RedisAddr     string
	MinioEndpoint string
	MinioBucket   string
	MinioRootUser string
	MinioPassword string
	ElasticURL    []string
	ElasticIndex  string
	Terminate     func()
}

type TestApp struct {
	Engine   *gin.Engine
	Postgres *sqlx.DB
	Redis    *redis.Client
	Minio    *minio.Client
	Elastic  *elasticsearch.Client
	Bucket   string
	Index    string
}

func runMigrations(dbURL string, migrationsPath string) error {
	m, err := migrate.New(
		"file://"+migrationsPath,
		dbURL,
	)
	if err != nil {
		return err
	}
	err = m.Up()
	if err != nil && err != migrate.ErrNoChange {
		return err
	}
	return nil
}

func SetupTestContainers(t *testing.T) *TestEnv {
	ctx := context.Background()

	dbC, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: testcontainers.ContainerRequest{
			Image: "postgres:latest",
			Env: map[string]string{
				"POSTGRES_DB":       "testdb",
				"POSTGRES_USER":     "testuser",
				"POSTGRES_PASSWORD": "testpass",
			},
			ExposedPorts: []string{"5432/tcp"},
			WaitingFor:   wait.ForListeningPort("5432/tcp").WithStartupTimeout(60 * time.Second),
		},
		Started: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	require.NoError(t, err)
	dbHost, err := dbC.Host(ctx)
	if err != nil {
		t.Fatalf("failed to get postgres host: %v", err)
	}
	dbPort, err := dbC.MappedPort(ctx, "5432")
	if err != nil {
		t.Fatalf("failed to get postgres port: %v", err)
	}

	postgresURI := fmt.Sprintf("host=%s port=%s user=testuser dbname=testdb password=testpass sslmode=disable", dbHost, dbPort.Port())
	migrateURI := fmt.Sprintf(
		"postgres://%s:%s@%s:%s/%s?sslmode=disable",
		"testuser", "testpass", dbHost, dbPort.Port(), "testdb",
	)
	fmt.Printf("dbHost: %s, dbPort: %s\n", dbHost, dbPort.Port())

	redisC, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: testcontainers.ContainerRequest{
			Image:        "redis:latest",
			ExposedPorts: []string{"6379/tcp"},
			WaitingFor:   wait.ForListeningPort("6379/tcp").WithStartupTimeout(60 * time.Second),
		},
		Started: true,
	})

	require.NoError(t, err)
	redisPort, _ := redisC.MappedPort(ctx, "6379")
	redisHost, _ := redisC.Host(ctx)
	redisAddr := fmt.Sprintf("%s:%s", redisHost, redisPort.Port())
	fmt.Printf("redis URI: %v\n", redisAddr)

	minioC, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: testcontainers.ContainerRequest{
			Image:        "minio/minio:latest",
			Env:          map[string]string{"MINIO_ROOT_USER": "minio", "MINIO_ROOT_PASSWORD": "minio123"},
			ExposedPorts: []string{"9000/tcp"},
			Cmd:          []string{"server", "/data"},
			WaitingFor:   wait.ForListeningPort("9000/tcp").WithStartupTimeout(60 * time.Second),
		},
		Started: true,
	})
	require.NoError(t, err)
	minioPort, _ := minioC.MappedPort(ctx, "9000")
	minioHost, _ := minioC.Host(ctx)
	minioEndpoint := fmt.Sprintf("%s:%s", minioHost, minioPort.Port())
	minioBucket := "testchukki"
	minioRootUser := "minio"
	minioPassword := "minio123"
	fmt.Printf("minio URI: %v", minioEndpoint)

	elasticC, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: testcontainers.ContainerRequest{
			Image:        "docker.elastic.co/elasticsearch/elasticsearch:8.9.0",
			Env:          map[string]string{"discovery.type": "single-node", "xpack.security.enabled": "false"},
			ExposedPorts: []string{"9200/tcp"},
			WaitingFor:   wait.ForListeningPort("9200/tcp").WithStartupTimeout(60 * time.Second),
		},
		Started: true,
	})
	require.NoError(t, err)
	elasticPort, _ := elasticC.MappedPort(ctx, "9200")
	elasticHost, _ := elasticC.Host(ctx)
	elasticURL := []string{fmt.Sprintf("http://%s:%s", elasticHost, elasticPort.Port())}
	elasticIndex := "testindex"
	fmt.Printf("elastic URI: %v\n", elasticURL)

	terminate := func() {
		_ = dbC.Terminate(ctx)
		_ = redisC.Terminate(ctx)
		_ = minioC.Terminate(ctx)
		_ = elasticC.Terminate(ctx)
	}

	return &TestEnv{
		PostgresURI:   postgresURI,
		MigrateURI:    migrateURI,
		RedisAddr:     redisAddr,
		MinioEndpoint: minioEndpoint,
		MinioBucket:   minioBucket,
		MinioRootUser: minioRootUser,
		MinioPassword: minioPassword,
		ElasticURL:    elasticURL,
		ElasticIndex:  elasticIndex,
		Terminate:     terminate,
	}
}

func setupTestApp(t *testing.T, env *TestEnv) *TestApp {
	ctx := context.Background()

	postgres, err := database.NewPostgresDB(ctx, env.PostgresURI)
	require.NoError(t, err)

	redis, err := cache.NewRedisClient(ctx, config.RedisConfig{Host: env.RedisAddr})
	require.NoError(t, err)

	minio, err := s3.NewMinioClient(ctx, config.MinioConfig{
		Host:     env.MinioEndpoint,
		Bucket:   env.MinioBucket,
		Rootuser: env.MinioRootUser,
		Password: env.MinioPassword,
		Ssl:      false,
	}, 10)
	require.NoError(t, err)

	elastiCli, err := elastic.NewElasticClient(config.ElasticConfig{
		Addresses: env.ElasticURL,
		Index:     env.ElasticIndex,
	})
	require.NoError(t, err)

	err = runMigrations(env.MigrateURI, "../../../migrations")
	require.NoError(t, err)

	repos := repository.NewRepository(postgres.Client(), redis.Client(), minio.Client(),
		elastiCli.Client(), minio.Pool(), env.MinioBucket)
	logger := logging.GetLogger()
	services := service.NewService(repos, logger)
	h := handler.NewHandler(services, logger)

	r := gin.Default()
	r.POST("/pasta", h.AccessPostMiddleware(), h.CreatePastaHandler)
	return &TestApp{
		Engine:   r,
		Postgres: postgres.Client(),
		Redis:    redis.Client(),
		Minio:    minio.Client(),
		Elastic:  elastiCli.Client(),
		Bucket:   "testchukki",
		Index:    "testindex",
	}
}

func cleanupAll(t *testing.T, postgres *sqlx.DB, redis *redis.Client, s3 *minio.Client, elastic *elasticsearch.Client, bucket, elasticIndex string) {
	ctx := context.Background()

	_, err := postgres.Exec("TRUNCATE TABLE pastas, users RESTART IDENTITY CASCADE;")
	require.NoError(t, err)

	require.NoError(t, redis.FlushAll(ctx).Err())

	objectsCh := make(chan minio.ObjectInfo)
	go func() {
		defer close(objectsCh)
		for object := range s3.ListObjects(ctx, bucket, minio.ListObjectsOptions{Recursive: true}) {
			objectsCh <- object
		}
	}()
	for err := range s3.RemoveObjects(ctx, bucket, objectsCh, minio.RemoveObjectsOptions{}) {
		require.NoError(t, err.Err)
	}

	// очистка elastic
	_, err = elastic.Indices.Delete([]string{elasticIndex})
	require.NoError(t, err)
}

func isInDatabase(objectID string, client *sqlx.DB) (error, bool) {
	var exists bool
	err := client.Get(&exists, "SELECT EXISTS(SELECT 1 FROM pastas WHERE object_id = $1)", objectID)
	return err, exists
}

func isInS3(objectID, bucket string, client *minio.Client) error {
	ctx := context.Background()
	_, err := client.StatObject(ctx, bucket, objectID, minio.GetObjectOptions{})
	return err
}

func isTextInCache(hash string, client *redis.Client) error {
	ctx := context.Background()
	_, err := client.Get(ctx, fmt.Sprintf("text:%s", hash)).Result()
	return err
}

func TestCreatePastaHandler(t *testing.T) {
	env := SetupTestContainers(t)
	app := setupTestApp(t, env)

	t.Cleanup(func() { env.Terminate() })
	t.Cleanup(func() { cleanupAll(t, app.Postgres, app.Redis, app.Minio, app.Elastic, app.Bucket, app.Index) })

	tests := []struct {
		name           string
		inputBody      dto.RequestCreatePasta
		expectedStatus int
	}{
		{
			name: "Successfully created (only message)",
			inputBody: dto.RequestCreatePasta{
				Message: "Test Message #1",
			},
			expectedStatus: 201,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			body, _ := json.Marshal(tt.inputBody)

			req := httptest.NewRequest(http.MethodPost, "/pasta", bytes.NewBuffer(body))
			req.Header.Set("Content-Type", "application/json")

			w := httptest.NewRecorder()
			app.Engine.ServeHTTP(w, req)

			var resp dto.SuccessCreatePastaResponse
			err := json.Unmarshal(w.Body.Bytes(), &resp)
			require.NoError(t, err)

			hash := strings.TrimPrefix(resp.Link, "http://localhost:8080/receive/")

			err, exists := isInDatabase(resp.Metadata.Key, app.Postgres)
			require.NoError(t, err)
			require.True(t, exists)

			err = isInS3(resp.Metadata.Key, app.Bucket, app.Minio)
			require.NoError(t, err)

			err = isTextInCache(hash, app.Redis)
			require.NoError(t, err)

			require.Equal(t, tt.expectedStatus, w.Code)
			require.Equal(t, "File uploaded successfully", resp.Message)
			require.Equal(t, "plaintext", resp.Metadata.Language)
			require.Equal(t, "public", resp.Metadata.Visibility)
			require.NotEmpty(t, resp.Link)
			require.NotEmpty(t, resp.Metadata)
		})
	}
}
