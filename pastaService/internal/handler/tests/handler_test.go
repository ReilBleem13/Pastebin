package tests

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"pastebin/internal/config"
	customerrors "pastebin/internal/errors"
	"pastebin/internal/handler"
	"pastebin/internal/infrastructure/elastic"
	minioClient "pastebin/internal/infrastructure/minio"
	postgresClient "pastebin/internal/infrastructure/postgres"
	"pastebin/internal/models"
	"pastebin/internal/repository"
	"pastebin/internal/service"
	"pastebin/internal/utils"
	"pastebin/pkg/dto"
	"strings"
	"testing"
	"time"

	"github.com/elastic/go-elasticsearch/v8"
	"github.com/gin-gonic/gin"
	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
	"github.com/minio/minio-go/v7"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
	"github.com/theartofdevel/logging"
)

const pastaURL string = "http://localhost:10002/receive/"

var (
	testEnv *TestEnv
	testApp *TestApp
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
	Engine         *gin.Engine
	Postgres       *sqlx.DB
	Redis          *redis.Client
	Minio          *minio.Client
	Elastic        *elasticsearch.Client
	ElasticService *elastic.ElasticClient
	Bucket         string
	Index          string
	Services       *service.Service
	repos          *repository.Repository
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

func SetupTestContainers() (*TestEnv, error) {
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
		return nil, err
	}

	dbHost, err := dbC.Host(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get postgres host: %v", err)
	}
	dbPort, err := dbC.MappedPort(ctx, "5432")
	if err != nil {
		return nil, fmt.Errorf("failed to get postgres port: %v", err)
	}

	postgresURI := fmt.Sprintf("host=%s port=%s user=testuser dbname=testdb password=testpass sslmode=disable", dbHost, dbPort.Port())
	migrateURI := fmt.Sprintf(
		"postgres://%s:%s@%s:%s/%s?sslmode=disable",
		"testuser", "testpass", dbHost, dbPort.Port(), "testdb",
	)

	redisC, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: testcontainers.ContainerRequest{
			Image:        "redis:latest",
			ExposedPorts: []string{"6379/tcp"},
			WaitingFor:   wait.ForListeningPort("6379/tcp").WithStartupTimeout(60 * time.Second),
		},
		Started: true,
	})
	if err != nil {
		return nil, err
	}

	redisPort, _ := redisC.MappedPort(ctx, "6379")
	redisHost, _ := redisC.Host(ctx)
	redisAddr := fmt.Sprintf("%s:%s", redisHost, redisPort.Port())

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
	if err != nil {
		return nil, err
	}

	minioPort, _ := minioC.MappedPort(ctx, "9000")
	minioHost, _ := minioC.Host(ctx)
	minioEndpoint := fmt.Sprintf("%s:%s", minioHost, minioPort.Port())
	minioBucket := "testchukki"
	minioRootUser := "minio"
	minioPassword := "minio123"

	elasticC, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: testcontainers.ContainerRequest{
			Image: "docker.elastic.co/elasticsearch/elasticsearch:8.9.0",
			Env: map[string]string{
				"discovery.type":                       "single-node",
				"xpack.security.enabled":               "false",
				"xpack.security.transport.ssl.enabled": "false",
				"ES_JAVA_OPTS":                         "-Xms512m -Xmx512m",
			},
			ExposedPorts: []string{"9200/tcp"},
			WaitingFor:   wait.ForHTTP("/").WithPort("9200/tcp").WithStartupTimeout(120 * time.Second),
		},
		Started: true,
	})
	if err != nil {
		return nil, err
	}

	elasticPort, err := elasticC.MappedPort(ctx, "9200")
	if err != nil {
		return nil, fmt.Errorf("failed to get elasticsearch port: %w", err)
	}
	elasticHost, err := elasticC.Host(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed tp get elasticsearch host: %w", err)
	}
	elasticURL := []string{fmt.Sprintf("http://%s:%s", elasticHost, elasticPort.Port())}
	elasticIndex := "testindex"

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
	}, nil
}

func setupTestApp(env *TestEnv) (*TestApp, error) {
	ctx := context.Background()

	logging.NewLogger(
		logging.WithAddSource(false),
		logging.WithIsJSON(false),
		logging.WithLevel("debug"),
		logging.WithLogFilePath("logs"),
	)

	postgres, err := postgresClient.NewPostgresDB(ctx, env.PostgresURI)
	if err != nil {
		return nil, err
	}

	redis := redis.NewClient(&redis.Options{
		Addr: env.RedisAddr,
	})

	minio, err := minioClient.NewMinioClient(ctx, config.MinioConfig{
		Addr:            env.MinioEndpoint,
		Bucket:          env.MinioBucket,
		User:            env.MinioRootUser,
		Password:        env.MinioPassword,
		MaxIdleConns:    100,
		IdleConnTimeout: 30 * time.Second,
		MaxRetries:      1,
		Ssl:             false,
	}, 10)
	if err != nil {
		return nil, err
	}

	elastiClient, err := elastic.NewElasticClient(ctx, config.ElasticConfig{
		Mode:               "debug",
		Addr:               env.ElasticURL[0],
		Username:           "",
		Password:           "",
		MaxRetries:         1,
		CompressRequstBody: true,
		Index:              env.ElasticIndex,
	})
	if err != nil {
		return nil, err
	}

	index, err := elastiClient.CreateSearchIndex(env.ElasticIndex)
	if err != nil {
		return nil, err
	}

	err = runMigrations(env.MigrateURI, "../../../../migrations")
	if err != nil {
		return nil, err
	}

	repos := repository.NewRepository(postgres.Client(), redis, minio.Client(),
		elastiClient.Client(), minio.Pool(), env.MinioBucket, index)
	services := service.NewService(ctx, repos)
	h := handler.NewHandler(ctx, services)

	gin.SetMode(gin.TestMode)
	r := gin.New()

	api := r.Group("/")
	api.Use(h.AuthMiddleWare())
	{
		api.POST("/create", h.AccessCreate(), h.CreatePastaHandler)
		api.GET("/receive/:hash", h.AccessHash(), h.GetPastaHandler)
		api.DELETE("/delete/:hash", h.AccessHash(), h.DeletePastaHandler)

		api.GET("/paginate", h.PaginatePublicHandler)
		api.GET("/search", h.SearchHandler)

		private := api.Group("/")
		private.Use(h.RequireAuth())
		{
			private.PUT("/update/:hash", h.UpdateHandler)
			private.GET("/paginate/me", h.PaginateForUserHandler)

			favorites := private.Group("/favorite")
			{
				favorites.GET("/:favorite_id", h.GetFavorite)
				favorites.DELETE("/:favorite_id", h.DeleteFavorite)
				favorites.POST("/create/:hash", h.CreateFavorite)
				favorites.GET("/paginate", h.PaginateFavorites)
			}
		}
	}
	return &TestApp{
		Engine:         r,
		Postgres:       postgres.Client(),
		Redis:          redis,
		Minio:          minio.Client(),
		Elastic:        elastiClient.Client(),
		ElasticService: elastiClient,
		Bucket:         env.MinioBucket,
		Index:          index,
		Services:       services,
		repos:          repos,
	}, nil
}

func cleanupAll(t *testing.T, postgres *sqlx.DB, redis *redis.Client, s3 *minio.Client, elasticService *elastic.ElasticClient, elastic *elasticsearch.Client, bucket, elasticIndex string) {
	ctx := context.Background()

	_, err := postgres.Exec("TRUNCATE TABLE pastas, users, favorites RESTART IDENTITY CASCADE;")
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

	_, err = elastic.Indices.Delete([]string{elasticIndex})
	require.NoError(t, err)

	newIndex, err := elasticService.CreateSearchIndex("testindex")
	require.NoError(t, err)
	testApp.Index = newIndex
}

func isInDatabase(objectID string, client *sqlx.DB) (error, bool) {
	var exists bool
	err := client.Get(&exists, "SELECT EXISTS(SELECT 1 FROM pastas WHERE object_id = $1)", objectID)
	return err, exists
}

func isDatabaseEmpty(client *sqlx.DB) (error, bool) {
	var exists bool
	err := client.Get(&exists, "SELECT EXISTS(SELECT 1 FROM pastas WHERE id > 0)")
	return err, exists
}

func isInS3(ctx context.Context, objectID, bucket string, client *minio.Client) error {
	_, err := client.StatObject(ctx, bucket, objectID, minio.GetObjectOptions{})
	return err
}

func isTextInCache(ctx context.Context, hash string, client *redis.Client) error {
	err := client.Get(ctx, fmt.Sprintf("text:%s", hash)).Err()
	if err == redis.Nil {
		log.Println("key is empty!")
		return nil
	} else if err != nil {
		return err
	}
	return nil
}

func isViewsInCache(ctx context.Context, hash string, client *redis.Client) error {
	return client.Get(ctx, fmt.Sprintf("views:%s", hash)).Err()
}

func TestMain(m *testing.M) {
	_ = os.WriteFile("config.yml", []byte("jwt:\n key: test-key\n"), 0644)
	defer os.Remove("config.yml")

	var err error
	testEnv, err = SetupTestContainers()
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to setup containers: %v\n", err)
		os.Exit(1)
	}
	testApp, err = setupTestApp(testEnv)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to setup test app: %v\n", err)
		os.Exit(1)
	}

	code := m.Run()
	testEnv.Terminate()
	os.Exit(code)
}

func TestCreatePastaHandler(t *testing.T) {
	tests := []struct {
		name           string
		inputBody      dto.RequestCreatePasta
		expectedStatus int
		withAuth       bool
		userID         int
		checkResponse  func(t *testing.T, resp *dto.SuccessCreatePastaResponse, w *httptest.ResponseRecorder, app *TestApp)
	}{
		{
			name: "Successfully Created (only message)",
			inputBody: dto.RequestCreatePasta{
				Message: "Test Message #1",
			},
			expectedStatus: 201,
			withAuth:       false,
			checkResponse: func(t *testing.T, resp *dto.SuccessCreatePastaResponse, w *httptest.ResponseRecorder, app *TestApp) {
				require.Equal(t, "File uploaded successfully", resp.Message)
				require.Equal(t, "plaintext", resp.Metadata.Language)
				require.Equal(t, models.VisibilityPublic, resp.Metadata.Visibility)
				require.NotEmpty(t, resp.Link)
				require.NotEmpty(t, resp.Metadata)

				hash := strings.TrimPrefix(resp.Link, pastaURL)
				err, exists := isInDatabase(resp.Metadata.Key, app.Postgres)
				require.NoError(t, err)
				require.True(t, exists)

				require.NoError(t, isInS3(t.Context(), resp.Metadata.Key, app.Bucket, app.Minio))
				require.NoError(t, isTextInCache(t.Context(), hash, app.Redis))
			},
		},
		{
			name: "Successfully Created (with all argumets)",
			inputBody: dto.RequestCreatePasta{
				Message:    "Test Message #1",
				Language:   "go",
				Visibility: "public",
				Expiration: "1w",
				Password:   "12345",
			},
			expectedStatus: 201,
			withAuth:       false,
			checkResponse: func(t *testing.T, resp *dto.SuccessCreatePastaResponse, w *httptest.ResponseRecorder, app *TestApp) {
				require.Equal(t, "File uploaded successfully", resp.Message)
				require.Equal(t, "go", resp.Metadata.Language)
				require.Equal(t, models.VisibilityPublic, resp.Metadata.Visibility)
				require.Equal(t, resp.Metadata.CreatedAt.Add(7*24*time.Hour), resp.Metadata.ExpiresAt)
				require.NotEmpty(t, resp.Link)
				require.NotEmpty(t, resp.Metadata)

				hash := strings.TrimPrefix(resp.Link, pastaURL)
				err, exists := isInDatabase(resp.Metadata.Key, app.Postgres)
				require.NoError(t, err)
				require.True(t, exists)

				require.NoError(t, isInS3(t.Context(), resp.Metadata.Key, app.Bucket, app.Minio))
				require.NoError(t, isTextInCache(t.Context(), hash, app.Redis))
			},
		},
		{
			name: "Successfully Created Private Pasta(with all argumets)",
			inputBody: dto.RequestCreatePasta{
				Message:    "Test Message #1",
				Language:   "go",
				Visibility: "private",
				Expiration: "1w",
				Password:   "12345",
			},
			expectedStatus: 201,
			withAuth:       true,
			userID:         1,
			checkResponse: func(t *testing.T, resp *dto.SuccessCreatePastaResponse, w *httptest.ResponseRecorder, app *TestApp) {
				require.Equal(t, "File uploaded successfully", resp.Message)
				require.Equal(t, "go", resp.Metadata.Language)
				require.Equal(t, models.VisibilityPrivate, resp.Metadata.Visibility)
				require.Equal(t, resp.Metadata.CreatedAt.Add(7*24*time.Hour), resp.Metadata.ExpiresAt)
				require.NotEmpty(t, resp.Link)
				require.NotEmpty(t, resp.Metadata)

				hash := strings.TrimPrefix(resp.Link, pastaURL)
				err, exists := isInDatabase(resp.Metadata.Key, app.Postgres)
				require.NoError(t, err)
				require.True(t, exists)

				require.NoError(t, isInS3(t.Context(), resp.Metadata.Key, app.Bucket, app.Minio))
				require.NoError(t, isTextInCache(t.Context(), hash, app.Redis))
			},
		},
		{
			name: "Invalid Expiration Format",
			inputBody: dto.RequestCreatePasta{
				Message:    "Test Message #1",
				Expiration: "1hour",
			},
			expectedStatus: 400,
			withAuth:       false,
			checkResponse: func(t *testing.T, resp *dto.SuccessCreatePastaResponse, w *httptest.ResponseRecorder, app *TestApp) {
				require.Contains(t, w.Body.String(), "invalid expiration format")
			},
		},
		{
			name: "Invalid Language Format",
			inputBody: dto.RequestCreatePasta{
				Message:  "Test Message #1",
				Language: "god-go",
			},
			expectedStatus: 400,
			withAuth:       false,
			checkResponse: func(t *testing.T, resp *dto.SuccessCreatePastaResponse, w *httptest.ResponseRecorder, app *TestApp) {
				require.Contains(t, w.Body.String(), "invalid language format")
			},
		},
		{
			name: "Invalid Visibility Format",
			inputBody: dto.RequestCreatePasta{
				Message:    "Test Message #1",
				Visibility: "Empty",
			},
			expectedStatus: 400,
			withAuth:       false,
			checkResponse: func(t *testing.T, resp *dto.SuccessCreatePastaResponse, w *httptest.ResponseRecorder, app *TestApp) {
				require.Contains(t, w.Body.String(), "invalid visibility format")
			},
		},
		{
			name: "Unathorized",
			inputBody: dto.RequestCreatePasta{
				Message:    "Test Message #1",
				Visibility: "private",
			},
			expectedStatus: 401,
			withAuth:       false,
			checkResponse: func(t *testing.T, resp *dto.SuccessCreatePastaResponse, w *httptest.ResponseRecorder, app *TestApp) {
				require.Contains(t, w.Body.String(), "user is not authenticated")
			},
		},
		{
			name:           "Invalid Request",
			inputBody:      dto.RequestCreatePasta{},
			expectedStatus: 400,
			withAuth:       false,
			checkResponse: func(t *testing.T, resp *dto.SuccessCreatePastaResponse, w *httptest.ResponseRecorder, app *TestApp) {
				require.Contains(t, w.Body.String(), "invalid request")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Cleanup(func() {
				cleanupAll(t, testApp.Postgres, testApp.Redis, testApp.Minio, testApp.ElasticService, testApp.Elastic, testApp.Bucket, testApp.Index)
			})

			body, err := json.Marshal(tt.inputBody)
			require.NoError(t, err)

			req := httptest.NewRequest(http.MethodPost, "/create", bytes.NewBuffer(body))
			req.Header.Set("Content-Type", "application/json")

			if tt.withAuth {
				token, err := utils.GenerateTestJWT(tt.userID)
				require.NoError(t, err)
				req.Header.Set("Authorization", "Bearer "+token)
			}

			w := httptest.NewRecorder()
			testApp.Engine.ServeHTTP(w, req)

			var resp dto.SuccessCreatePastaResponse
			_ = json.Unmarshal(w.Body.Bytes(), &resp)

			require.Equal(t, tt.expectedStatus, w.Code)
			tt.checkResponse(t, &resp, w, testApp)
		})
	}
}

func TestGetPastaHandler(t *testing.T) {
	tests := []struct {
		name           string
		inputBody      *dto.Password
		customBody     []byte
		withPasta      bool
		testPasta      *dto.RequestCreatePasta
		url            func(urlRrefix string, hash string) string
		expectedStatus int
		withAuth       bool
		userID         int
		userForJWT     int
		query          string
		checkResponse  func(t *testing.T, resp *dto.GetPastaResponse, w *httptest.ResponseRecorder, app *TestApp)
	}{
		{
			name:      "Successfully Got",
			withPasta: true,
			testPasta: &dto.RequestCreatePasta{
				Message:    "Test Message #1",
				Language:   "go",
				Expiration: "1d",
			},
			url: func(urlRrefix string, hash string) string {
				return urlRrefix + "/" + hash
			},
			expectedStatus: 200,
			checkResponse: func(t *testing.T, resp *dto.GetPastaResponse, w *httptest.ResponseRecorder, app *TestApp) {
				require.Equal(t, "Successfully got pasta", resp.Message)
				require.Equal(t, "Test Message #1", resp.Text)
			},
		},
		{
			name: "Successfully Got (with Password)",
			inputBody: &dto.Password{
				Password: "12345",
			},
			withPasta: true,
			testPasta: &dto.RequestCreatePasta{
				Message:    "Test Message #1",
				Language:   "go",
				Expiration: "1d",
				Password:   "12345",
			},
			url: func(urlRrefix string, hash string) string {
				return urlRrefix + "/" + hash
			},
			expectedStatus: 200,
			checkResponse: func(t *testing.T, resp *dto.GetPastaResponse, w *httptest.ResponseRecorder, app *TestApp) {
				require.Equal(t, "Successfully got pasta", resp.Message)
				require.Equal(t, "Test Message #1", resp.Text)
			},
		},
		{
			name:      "Successfully Got (metadata)",
			withPasta: true,
			testPasta: &dto.RequestCreatePasta{
				Message:    "Test Message #1",
				Language:   "go",
				Expiration: "1d",
			},
			url: func(urlRrefix string, hash string) string {
				return urlRrefix + "/" + hash
			},
			expectedStatus: 200,
			query:          "metadata=true",
			checkResponse: func(t *testing.T, resp *dto.GetPastaResponse, w *httptest.ResponseRecorder, app *TestApp) {
				require.Equal(t, "Successfully got pasta", resp.Message)
				require.Equal(t, "Test Message #1", resp.Text)

				require.Equal(t, 0, resp.Metadata.UserID)
				require.Equal(t, "go", resp.Metadata.Language)
				require.Equal(t, models.VisibilityPublic, resp.Metadata.Visibility)
				require.Equal(t, 1, resp.Metadata.Views)
				require.Equal(t, resp.Metadata.CreatedAt.Add(24*time.Hour), resp.Metadata.ExpiresAt)
			},
		},
		{
			name: "Successfully Got (metadata + password)",
			inputBody: &dto.Password{
				Password: "12345",
			},
			withPasta: true,
			testPasta: &dto.RequestCreatePasta{
				Message:    "Test Message #1",
				Language:   "go",
				Expiration: "1d",
				Password:   "12345",
			},
			url: func(urlRrefix string, hash string) string {
				return urlRrefix + "/" + hash
			},
			expectedStatus: 200,
			query:          "metadata=true",
			checkResponse: func(t *testing.T, resp *dto.GetPastaResponse, w *httptest.ResponseRecorder, app *TestApp) {
				require.Equal(t, "Successfully got pasta", resp.Message)
				require.Equal(t, "Test Message #1", resp.Text)

				require.Equal(t, 0, resp.Metadata.UserID)
				require.Equal(t, "go", resp.Metadata.Language)
				require.Equal(t, models.VisibilityPublic, resp.Metadata.Visibility)
				require.Equal(t, 1, resp.Metadata.Views)
				require.Equal(t, resp.Metadata.CreatedAt.Add(24*time.Hour), resp.Metadata.ExpiresAt)
			},
		},
		{
			name: "Successfully Got by User(metadata)",
			inputBody: &dto.Password{
				Password: "12345",
			},
			withPasta: true,
			testPasta: &dto.RequestCreatePasta{
				Message:    "Test Message #1",
				Language:   "go",
				Expiration: "1d",
				Password:   "12345",
			},
			url: func(urlRrefix string, hash string) string {
				return urlRrefix + "/" + hash
			},
			expectedStatus: 200,
			userID:         1,
			query:          "metadata=true",
			checkResponse: func(t *testing.T, resp *dto.GetPastaResponse, w *httptest.ResponseRecorder, app *TestApp) {
				require.Equal(t, "Successfully got pasta", resp.Message)
				require.Equal(t, "Test Message #1", resp.Text)
				require.Equal(t, 1, resp.Metadata.UserID)
				require.Equal(t, "go", resp.Metadata.Language)
				require.Equal(t, models.VisibilityPublic, resp.Metadata.Visibility)
				require.Equal(t, 1, resp.Metadata.Views)
				require.Equal(t, resp.Metadata.CreatedAt.Add(24*time.Hour), resp.Metadata.ExpiresAt)
			},
		},
		{
			name: "Successfully Got Private (metadata)",
			inputBody: &dto.Password{
				Password: "12345",
			},
			withPasta: true,
			testPasta: &dto.RequestCreatePasta{
				Message:    "Test Message #1",
				Language:   "go",
				Expiration: "1d",
				Visibility: "private",
				Password:   "12345",
			},
			url: func(urlRrefix string, hash string) string {
				return urlRrefix + "/" + hash
			},
			expectedStatus: 200,
			withAuth:       true,
			userID:         1,
			userForJWT:     1,
			query:          "metadata=true",
			checkResponse: func(t *testing.T, resp *dto.GetPastaResponse, w *httptest.ResponseRecorder, app *TestApp) {
				require.Equal(t, "Successfully got pasta", resp.Message)
				require.Equal(t, "Test Message #1", resp.Text)
				require.Equal(t, 1, resp.Metadata.UserID)
				require.Equal(t, "go", resp.Metadata.Language)
				require.Equal(t, models.VisibilityPrivate, resp.Metadata.Visibility)
				require.Equal(t, 1, resp.Metadata.Views)
				require.Equal(t, resp.Metadata.CreatedAt.Add(24*time.Hour), resp.Metadata.ExpiresAt)
			},
		},
		{
			name: "Pasta Not Found",
			url: func(urlRrefix string, _ string) string {
				return urlRrefix + "/" + "notfoundpasta"
			},
			expectedStatus: 404,
			checkResponse: func(t *testing.T, resp *dto.GetPastaResponse, w *httptest.ResponseRecorder, app *TestApp) {
				require.Contains(t, w.Body.String(), "pasta is not found")
			},
		},
		{
			name: "Wrong Password",
			inputBody: &dto.Password{
				Password: "12345",
			},
			withPasta: true,
			testPasta: &dto.RequestCreatePasta{
				Message:  "Test Message #1",
				Password: "67890",
			},
			url: func(urlRrefix string, hash string) string {
				return urlRrefix + "/" + hash
			},
			expectedStatus: 403,
			checkResponse: func(t *testing.T, resp *dto.GetPastaResponse, w *httptest.ResponseRecorder, app *TestApp) {
				require.Contains(t, w.Body.String(), "password is wrong")
			},
		},
		{
			name:      "No Access",
			withPasta: true,
			testPasta: &dto.RequestCreatePasta{
				Message:    "Test Message #1",
				Visibility: "private",
			},
			url: func(urlRrefix string, hash string) string {
				return urlRrefix + "/" + hash
			},
			expectedStatus: 403,
			withAuth:       true,
			userID:         1,
			userForJWT:     2,
			checkResponse: func(t *testing.T, resp *dto.GetPastaResponse, w *httptest.ResponseRecorder, app *TestApp) {
				require.Contains(t, w.Body.String(), "no access")
			},
		},
		{
			name:      "Unauthorized",
			withPasta: true,
			testPasta: &dto.RequestCreatePasta{
				Message:    "Test Message #1",
				Visibility: "private",
			},
			url: func(urlRrefix string, hash string) string {
				return urlRrefix + "/" + hash
			},
			expectedStatus: 401,
			checkResponse: func(t *testing.T, resp *dto.GetPastaResponse, w *httptest.ResponseRecorder, app *TestApp) {
				require.Contains(t, w.Body.String(), "user is not authenticated")
			},
		},
		{
			name:      "Invalid Query Parament",
			withPasta: true,
			testPasta: &dto.RequestCreatePasta{
				Message: "Test Message #1",
			},
			url: func(urlRrefix string, hash string) string {
				return urlRrefix + "/" + hash
			},
			expectedStatus: 400,
			userID:         1,
			query:          "metadata=invalid",
			checkResponse: func(t *testing.T, resp *dto.GetPastaResponse, w *httptest.ResponseRecorder, app *TestApp) {
				require.Contains(t, w.Body.String(), "valid 'metadata' query parameter, must be 'true' of 'false'")
			},
		},
		{
			name:      "Invalid Password JSON",
			withPasta: true,
			testPasta: &dto.RequestCreatePasta{
				Message: "Test Message #1",
			},
			url: func(urlRrefix string, hash string) string {
				return urlRrefix + "/" + hash
			},
			expectedStatus: 400,
			customBody:     []byte(`{invalid json}`),
			checkResponse: func(t *testing.T, resp *dto.GetPastaResponse, w *httptest.ResponseRecorder, app *TestApp) {
				require.Contains(t, w.Body.String(), "invalid character")
			},
		},
		{
			name:      "Expire after Read",
			withPasta: true,
			testPasta: &dto.RequestCreatePasta{
				Message:         "Message with expire after read",
				ExpireAfterRead: true,
			},
			url: func(urlRrefix, hash string) string {
				return urlRrefix + "/" + hash
			},
			expectedStatus: 200,
			checkResponse: func(t *testing.T, resp *dto.GetPastaResponse, w *httptest.ResponseRecorder, app *TestApp) {
				err, exists := isDatabaseEmpty(app.Postgres)
				require.NoError(t, err)
				require.False(t, exists)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Cleanup(func() {
				cleanupAll(t, testApp.Postgres, testApp.Redis, testApp.Minio, testApp.ElasticService, testApp.Elastic, testApp.Bucket, testApp.Index)
			})

			ctx := t.Context()

			var body []byte
			if tt.inputBody != nil {
				var err error
				body, err = json.Marshal(tt.inputBody)
				require.NoError(t, err)
			}

			pasta := &models.Pasta{}
			if tt.withPasta {
				var err error
				pasta, err = testApp.Services.Pasta.Create(ctx, tt.testPasta, tt.userID)
				require.NoError(t, err)
			}

			url := tt.url("/receive", pasta.Hash)
			if tt.query != "" {
				url += "?" + tt.query
			}

			var req *http.Request
			if tt.inputBody != nil {
				req = httptest.NewRequest(http.MethodGet, url, bytes.NewBuffer(body))
			} else if tt.customBody != nil {
				req = httptest.NewRequest(http.MethodGet, url, bytes.NewBuffer(tt.customBody))
			} else {
				req = httptest.NewRequest(http.MethodGet, url, nil) // если указать body, который является nil, то он будет расценен как пустой буфер.
			}
			req.Header.Set("Content-Type", "application/json")

			if tt.withAuth {
				token, err := utils.GenerateTestJWT(tt.userForJWT)
				require.NoError(t, err)
				req.Header.Set("Authorization", "Bearer "+token)
			}

			w := httptest.NewRecorder()
			testApp.Engine.ServeHTTP(w, req)

			var resp dto.GetPastaResponse
			_ = json.Unmarshal(w.Body.Bytes(), &resp)

			require.Equal(t, tt.expectedStatus, w.Code)
			tt.checkResponse(t, &resp, w, testApp)
		})
	}
}

func TestDeletePastaHandler(t *testing.T) {
	tests := []struct {
		name           string
		inputBody      *dto.Password
		withPasta      bool
		testPasta      *dto.RequestCreatePasta
		url            func(urlRrefix string, hash string) string
		expectedStatus int
		withAuth       bool
		userID         int
		userForJWT     int
		checkResponse  func(t *testing.T, w *httptest.ResponseRecorder, objectID, hash *string)
	}{
		{
			name: "Successfully deleted",
			inputBody: &dto.Password{
				Password: "123",
			},
			withPasta: true,
			testPasta: &dto.RequestCreatePasta{
				Message:  "Test Message #1",
				Password: "123",
			},
			url: func(urlRrefix, hash string) string {
				return urlRrefix + "/" + hash
			},
			expectedStatus: 200,
			checkResponse: func(t *testing.T, w *httptest.ResponseRecorder, objectID, hash *string) {
				require.Contains(t, w.Body.String(), "Deleted")

				err, exists := isInDatabase(*objectID, testApp.Postgres)
				require.NoError(t, err)
				require.False(t, exists)

				require.Error(t, isInS3(t.Context(), *objectID, testApp.Bucket, testApp.Minio))
				require.Error(t, isViewsInCache(t.Context(), *hash, testApp.Redis))
			},
		},
		{
			name:      "Successfully deleted Private",
			withPasta: true,
			testPasta: &dto.RequestCreatePasta{
				Message:    "Test Message #1",
				Visibility: "private",
			},
			url: func(urlRrefix, hash string) string {
				return urlRrefix + "/" + hash
			},
			withAuth:       true,
			userID:         1,
			userForJWT:     1,
			expectedStatus: 200,
			checkResponse: func(t *testing.T, w *httptest.ResponseRecorder, objectID, hash *string) {
				require.Contains(t, w.Body.String(), "Deleted")

				err, exists := isInDatabase(*objectID, testApp.Postgres)
				require.NoError(t, err)
				require.False(t, exists)

				require.Error(t, isInS3(t.Context(), *objectID, testApp.Bucket, testApp.Minio))
				require.Error(t, isViewsInCache(t.Context(), *hash, testApp.Redis))
			},
		},
		{
			name: "Pasta Not Found",
			url: func(urlRrefix, hash string) string {
				return urlRrefix + "/" + "test"
			},
			expectedStatus: 404,
			checkResponse: func(t *testing.T, w *httptest.ResponseRecorder, objectID, hash *string) {
				require.Contains(t, w.Body.String(), customerrors.ErrPastaNotFound.Error())
			},
		},
		{
			name:      "Unauthorized",
			withPasta: true,
			testPasta: &dto.RequestCreatePasta{
				Message:    "Test Message #1",
				Visibility: "private",
			},
			url: func(urlRrefix, hash string) string {
				return urlRrefix + "/" + hash
			},
			expectedStatus: 401,
			userID:         1,
			checkResponse: func(t *testing.T, w *httptest.ResponseRecorder, objectID, hash *string) {
				require.Contains(t, w.Body.String(), customerrors.ErrUserNotAuthenticated.Error())
			},
		},
		{
			name:      "No Access",
			withPasta: true,
			testPasta: &dto.RequestCreatePasta{
				Message:    "Test Message #1",
				Visibility: "private",
			},
			url: func(urlRrefix, hash string) string {
				return urlRrefix + "/" + hash
			},
			expectedStatus: 403,
			withAuth:       true,
			userID:         1,
			userForJWT:     2,
			checkResponse: func(t *testing.T, w *httptest.ResponseRecorder, objectID, hash *string) {
				require.Contains(t, w.Body.String(), customerrors.ErrNoAccess.Error())
			},
		},
		{
			name: "Wrong Password",
			inputBody: &dto.Password{
				Password: "1222333",
			},
			withPasta: true,
			testPasta: &dto.RequestCreatePasta{
				Message:  "Test Message #1",
				Password: "123",
			},
			url: func(urlRrefix, hash string) string {
				return urlRrefix + "/" + hash
			},
			expectedStatus: 403,
			withAuth:       true,
			userID:         1,
			userForJWT:     1,
			checkResponse: func(t *testing.T, w *httptest.ResponseRecorder, objectID, hash *string) {
				require.Contains(t, w.Body.String(), customerrors.ErrWrongPassword.Error())
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Cleanup(func() {
				cleanupAll(t, testApp.Postgres, testApp.Redis, testApp.Minio, testApp.ElasticService, testApp.Elastic, testApp.Bucket, testApp.Index)
			})

			ctx := t.Context()

			var body []byte
			if tt.inputBody != nil {
				var err error
				body, err = json.Marshal(tt.inputBody)
				require.NoError(t, err)
			}

			pasta := &models.Pasta{}
			if tt.withPasta {
				var err error
				pasta, err = testApp.Services.Pasta.Create(ctx, tt.testPasta, tt.userID)
				require.NoError(t, err)
			}

			url := tt.url("/delete", pasta.Hash)

			var req *http.Request
			if tt.inputBody != nil {
				req = httptest.NewRequest(http.MethodDelete, url, bytes.NewBuffer(body))
			} else {
				req = httptest.NewRequest(http.MethodDelete, url, nil)
			}
			req.Header.Set("Content-Type", "application/json")

			if tt.withAuth {
				token, err := utils.GenerateTestJWT(tt.userForJWT)
				require.NoError(t, err)
				req.Header.Set("Authorization", "Bearer "+token)
			}

			w := httptest.NewRecorder()
			testApp.Engine.ServeHTTP(w, req)

			require.Equal(t, tt.expectedStatus, w.Code)
			tt.checkResponse(t, w, &pasta.ObjectID, &pasta.Hash)
		})
	}
}

func TestPaginatePublicPastaHandler(t *testing.T) {
	tests := []struct {
		name           string
		withPasta      bool
		testPastas     []dto.RequestCreatePasta
		expectedStatus int
		userID         int
		query          string
		checkResponse  func(t *testing.T, w *httptest.ResponseRecorder, resp *dto.PaginatedPastaDTO, testPastas []dto.RequestCreatePasta)
	}{
		{
			name:      "Success Paginated (without metadata)",
			withPasta: true,
			testPastas: []dto.RequestCreatePasta{
				{Message: "First message"},
				{Message: "Second message"},
			},
			expectedStatus: 200,
			query:          "?limit=5&page=1",
			checkResponse: func(t *testing.T, w *httptest.ResponseRecorder, resp *dto.PaginatedPastaDTO, testPastas []dto.RequestCreatePasta) {
				require.Equal(t, 5, resp.Limit)
				require.Equal(t, 1, resp.Page)
				require.Equal(t, 2, resp.Total)
				require.Len(t, resp.Pastas, len(testPastas))

				expectedTexts := make(map[string]bool, len(testPastas))
				for _, p := range testPastas {
					expectedTexts[p.Message] = true
				}
				for _, actual := range resp.Pastas {
					require.True(t, expectedTexts[actual.Text])
				}
			},
		},
		{
			name:      "Success Paginated (with metadata)",
			withPasta: true,
			testPastas: []dto.RequestCreatePasta{
				{Message: "First message"},
				{Message: "Second message"},
			},
			expectedStatus: 200,
			query:          "?limit=5&page=1&metadata=true",
			checkResponse: func(t *testing.T, w *httptest.ResponseRecorder, resp *dto.PaginatedPastaDTO, testPastas []dto.RequestCreatePasta) {
				require.Equal(t, 5, resp.Limit)
				require.Equal(t, 1, resp.Page)
				require.Equal(t, 2, resp.Total)
				require.Len(t, resp.Pastas, len(testPastas))

				expectedTexts := make(map[string]bool, len(testPastas))
				for _, p := range testPastas {
					expectedTexts[p.Message] = true
				}

				for _, actual := range resp.Pastas {
					require.True(t, expectedTexts[actual.Text])
					require.NotEmpty(t, actual.Metadata)
				}
			},
		},
		{
			name:           "Pasta Not Found",
			expectedStatus: 200,
			checkResponse: func(t *testing.T, w *httptest.ResponseRecorder, resp *dto.PaginatedPastaDTO, testPastas []dto.RequestCreatePasta) {
				require.Contains(t, w.Body.String(), customerrors.ErrPastaNotFound.Error())
			},
		},
		{
			name:           "Invalid Query Parametr",
			query:          "?limit=ad",
			expectedStatus: 400,
			checkResponse: func(t *testing.T, w *httptest.ResponseRecorder, resp *dto.PaginatedPastaDTO, testPastas []dto.RequestCreatePasta) {
				require.Contains(t, w.Body.String(), customerrors.ErrInvalidQueryParament.Error())
			},
		},
		{
			name:      "Exist Only Private Pastas",
			withPasta: true,
			testPastas: []dto.RequestCreatePasta{
				{
					Message:    "Private Pasta #1",
					Visibility: "private",
				},
				{
					Message:    "Private Pasta #2",
					Visibility: "private",
				},
			},
			userID:         1,
			expectedStatus: 200,
			checkResponse: func(t *testing.T, w *httptest.ResponseRecorder, resp *dto.PaginatedPastaDTO, testPastas []dto.RequestCreatePasta) {
				require.Contains(t, w.Body.String(), customerrors.ErrPastaNotFound.Error())
			},
		},
		{
			name:      "Exists Only Public by User",
			withPasta: true,
			testPastas: []dto.RequestCreatePasta{
				{
					Message: "Private Pasta #1",
				},
				{
					Message: "Private Pasta #2",
				},
			},
			userID:         1,
			expectedStatus: 200,
			checkResponse: func(t *testing.T, w *httptest.ResponseRecorder, resp *dto.PaginatedPastaDTO, testPastas []dto.RequestCreatePasta) {
				require.Equal(t, 5, resp.Limit)
				require.Equal(t, 1, resp.Page)
				require.Equal(t, 2, resp.Total)
				require.Len(t, resp.Pastas, len(testPastas))

				expectedTexts := make(map[string]bool, len(testPastas))
				for _, p := range testPastas {
					expectedTexts[p.Message] = true
				}

				for _, actual := range resp.Pastas {
					require.True(t, expectedTexts[actual.Text])
				}
			},
		},
		{
			name:      "Exists Only With Password",
			withPasta: true,
			testPastas: []dto.RequestCreatePasta{
				{
					Message:  "Private Pasta #1",
					Password: "123",
				},
				{
					Message:  "Private Pasta #2",
					Password: "123",
				},
			},
			expectedStatus: 200,
			checkResponse: func(t *testing.T, w *httptest.ResponseRecorder, resp *dto.PaginatedPastaDTO, testPastas []dto.RequestCreatePasta) {
				require.Contains(t, w.Body.String(), customerrors.ErrPastaNotFound.Error())
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Cleanup(func() {
				cleanupAll(t, testApp.Postgres, testApp.Redis, testApp.Minio, testApp.ElasticService, testApp.Elastic, testApp.Bucket, testApp.Index)
			})

			ctx := t.Context()

			if tt.withPasta {
				for _, pasta := range tt.testPastas {
					_, err := testApp.Services.Pasta.Create(ctx, &pasta, tt.userID)
					require.NoError(t, err)
				}
			}

			url := "/paginate"
			if tt.query != "" {
				url += tt.query
			}

			req := httptest.NewRequest(http.MethodGet, url, nil)
			req.Header.Set("Content-Type", "application/json")

			w := httptest.NewRecorder()
			testApp.Engine.ServeHTTP(w, req)

			var resp dto.PaginatedPastaDTO
			_ = json.Unmarshal(w.Body.Bytes(), &resp)

			require.Equal(t, tt.expectedStatus, w.Code)
			tt.checkResponse(t, w, &resp, tt.testPastas)
		})
	}
}

type TestPastaWithUserID struct {
	Pasta  dto.RequestCreatePasta
	UserID int
}

func TestPaginateForUserPastaHandler(t *testing.T) {
	tests := []struct {
		name           string
		withPasta      bool
		testPastas     []TestPastaWithUserID
		expectedStatus int
		withAuth       bool
		userForJWT     int
		query          string
		checkResponse  func(t *testing.T, w *httptest.ResponseRecorder, resp *dto.PaginatedPastaDTO, testPastas []TestPastaWithUserID)
	}{
		{
			name:      "Success Paginate",
			withPasta: true,
			testPastas: []TestPastaWithUserID{
				{
					Pasta: dto.RequestCreatePasta{
						Message: "First message",
					},
					UserID: 1,
				},
				{
					Pasta: dto.RequestCreatePasta{
						Message: "Second message",
					},
					UserID: 1,
				},
			},
			withAuth:       true,
			userForJWT:     1,
			expectedStatus: 200,
			checkResponse: func(t *testing.T, w *httptest.ResponseRecorder, resp *dto.PaginatedPastaDTO, testPastas []TestPastaWithUserID) {
				require.Len(t, resp.Pastas, 2)

				expectedTexts := make(map[string]bool, len(testPastas))
				for _, p := range testPastas {
					expectedTexts[p.Pasta.Message] = true
				}

				for _, actual := range resp.Pastas {
					require.True(t, expectedTexts[actual.Text])
				}
			},
		},
		{
			name:      "Success Paginate (with Metadata)",
			withPasta: true,
			testPastas: []TestPastaWithUserID{
				{
					Pasta: dto.RequestCreatePasta{
						Message: "First message",
					},
					UserID: 1,
				},
				{
					Pasta: dto.RequestCreatePasta{
						Message: "Second message",
					},
					UserID: 1,
				},
			},
			withAuth:       true,
			userForJWT:     1,
			expectedStatus: 200,
			query:          "?limit=10&page=1",
			checkResponse: func(t *testing.T, w *httptest.ResponseRecorder, resp *dto.PaginatedPastaDTO, testPastas []TestPastaWithUserID) {
				require.Equal(t, 10, resp.Limit)
				require.Equal(t, 1, resp.Page)
				require.Len(t, resp.Pastas, 2)

				expectedTexts := make(map[string]bool, len(testPastas))
				for _, p := range testPastas {
					expectedTexts[p.Pasta.Message] = true
				}

				for _, actual := range resp.Pastas {
					require.True(t, expectedTexts[actual.Text])
					require.Nil(t, actual.Metadata)
				}
			},
		},
		{
			name:      "Two Pastas Two User",
			withPasta: true,
			testPastas: []TestPastaWithUserID{
				{
					Pasta: dto.RequestCreatePasta{
						Message: "First message (first user)",
					},
					UserID: 1,
				},
				{
					Pasta: dto.RequestCreatePasta{
						Message: "Second message (second user)",
					},
					UserID: 2,
				},
			},
			withAuth:       true,
			userForJWT:     1,
			expectedStatus: 200,
			query:          "?limit=10&page=1",
			checkResponse: func(t *testing.T, w *httptest.ResponseRecorder, resp *dto.PaginatedPastaDTO, testPastas []TestPastaWithUserID) {
				require.Equal(t, 10, resp.Limit)
				require.Equal(t, 1, resp.Page)
				require.Len(t, resp.Pastas, 1)

				for _, actual := range resp.Pastas {
					require.Contains(t, actual.Text, "First")
					require.Nil(t, actual.Metadata)
				}
			},
		},
		{
			name:           "Not Authorized",
			expectedStatus: 401,
			checkResponse: func(t *testing.T, w *httptest.ResponseRecorder, resp *dto.PaginatedPastaDTO, testPastas []TestPastaWithUserID) {
				require.Contains(t, w.Body.String(), customerrors.ErrUserNotAuthenticated.Error())
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Cleanup(func() {
				cleanupAll(t, testApp.Postgres, testApp.Redis, testApp.Minio, testApp.ElasticService, testApp.Elastic, testApp.Bucket, testApp.Index)
			})

			ctx := t.Context()

			if tt.withPasta {
				for _, pasta := range tt.testPastas {
					_, err := testApp.Services.Pasta.Create(ctx, &pasta.Pasta, pasta.UserID)
					require.NoError(t, err)
				}
			}

			url := "/paginate/me"
			if tt.query != "" {
				url += tt.query
			}

			req := httptest.NewRequest(http.MethodGet, url, nil)
			req.Header.Set("Content-Type", "application/json")

			if tt.withAuth {
				token, err := utils.GenerateTestJWT(tt.userForJWT)
				require.NoError(t, err)
				req.Header.Set("Authorization", "Bearer "+token)
			}
			log.Println(req.URL.String())
			w := httptest.NewRecorder()
			testApp.Engine.ServeHTTP(w, req)

			resp := &dto.PaginatedPastaDTO{}
			_ = json.Unmarshal(w.Body.Bytes(), &resp)

			require.Equal(t, tt.expectedStatus, w.Code)
			tt.checkResponse(t, w, resp, tt.testPastas)
		})
	}
}

var UrlForGet string = "localhost:10002/receive/"

func TestSearchPastaHandle(t *testing.T) {
	tests := []struct {
		name           string
		withPasta      bool
		expectedStatus int
		testsPasta     []dto.RequestCreatePasta
		query          string
		checkResponse  func(t *testing.T, w *httptest.ResponseRecorder, resp *dto.SearchedPastas, hashs []string)
	}{
		{
			name:           "Success",
			withPasta:      true,
			expectedStatus: 200,
			testsPasta: []dto.RequestCreatePasta{
				{Message: "Примерный жираф"},
				{Message: "Примерный кот"},
			},
			query: "?search=жираф",
			checkResponse: func(t *testing.T, w *httptest.ResponseRecorder, resp *dto.SearchedPastas, hashs []string) {
				require.Len(t, hashs, 2)

				expectedHashs := make(map[string]bool, len(hashs))
				for _, hash := range hashs {
					expectedHashs[hash] = true
				}

				for _, rawPasta := range resp.Pastas {
					data := strings.TrimPrefix(rawPasta, UrlForGet)
					require.True(t, expectedHashs[data])
				}
			},
		},
		{
			name:           "Empty Search Result",
			expectedStatus: 200,
			query:          "?search=жираф",
			checkResponse: func(t *testing.T, w *httptest.ResponseRecorder, resp *dto.SearchedPastas, hashs []string) {
				require.Contains(t, w.Body.String(), customerrors.ErrEmptySearchResult.Error())
			},
		},
		{
			name:           "Empty Search Field",
			expectedStatus: 400,
			query:          "?search=",
			checkResponse: func(t *testing.T, w *httptest.ResponseRecorder, resp *dto.SearchedPastas, hashs []string) {
				require.Contains(t, w.Body.String(), customerrors.ErrEmptySearchField.Error())
			},
		},
		{
			name:      "Only 1/3 is public",
			withPasta: true,
			testsPasta: []dto.RequestCreatePasta{
				{Message: "First message", Visibility: "private"},
				{Message: "Second message", Visibility: "private"},
				{Message: "Third message"},
			},
			expectedStatus: 200,
			query:          "?search=message",
			checkResponse: func(t *testing.T, w *httptest.ResponseRecorder, resp *dto.SearchedPastas, hashs []string) {
				require.Len(t, hashs, 1)
				require.Equal(t, hashs[0], strings.TrimPrefix(resp.Pastas[0], UrlForGet))
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Cleanup(func() {
				cleanupAll(t, testApp.Postgres, testApp.Redis, testApp.Minio, testApp.ElasticService, testApp.Elastic, testApp.Bucket, testApp.Index)
			})

			ctx := t.Context()

			hashs := []string{}
			if tt.withPasta {
				for _, pasta := range tt.testsPasta {
					p, err := testApp.Services.Pasta.Create(ctx, &pasta, 0)
					require.NoError(t, err)
					if pasta.Visibility != "private" {
						hashs = append(hashs, p.Hash)
					}
				}
			}
			url := "/search" + tt.query

			req := httptest.NewRequest(http.MethodGet, url, nil)
			req.Header.Set("Content-Type", "application/json")

			w := httptest.NewRecorder()
			testApp.Engine.ServeHTTP(w, req)

			resp := &dto.SearchedPastas{}
			_ = json.Unmarshal(w.Body.Bytes(), &resp)

			require.Equal(t, tt.expectedStatus, w.Code)
			tt.checkResponse(t, w, resp, hashs)
		})
	}
}

func TestUpdatePastaHandler(t *testing.T) {
	tests := []struct {
		name          string
		withPasta     bool
		testPastas    TestPastaWithUserID
		withAuth      bool
		userIDForAuth int

		inputBody      dto.UpdateRequest
		expectedStatus int
		checkResponse  func(t *testing.T, w *httptest.ResponseRecorder, resp *dto.SuccessUpdatedPastaResponse, createdPasta *dto.TextsWithMetadata)
	}{
		{
			name:      "Success",
			withPasta: true,
			testPastas: TestPastaWithUserID{
				Pasta: dto.RequestCreatePasta{
					Message: "Message before update",
				},
				UserID: 1,
			},
			withAuth:      true,
			userIDForAuth: 1,
			inputBody: dto.UpdateRequest{
				NewText: "Message after update",
			},
			expectedStatus: 200,
			checkResponse: func(t *testing.T, w *httptest.ResponseRecorder, resp *dto.SuccessUpdatedPastaResponse, createdPasta *dto.TextsWithMetadata) {
				require.NotEqual(t, resp.Metadata.Size, createdPasta.Metadata.Size)

				require.Equal(t, resp.Metadata.Visibility, createdPasta.Metadata.Visibility)
				require.Equal(t, resp.Metadata.Views, createdPasta.Metadata.Views)
				require.Equal(t, resp.Metadata.UserID, createdPasta.Metadata.UserID)
				require.Equal(t, resp.Metadata.ObjectID, createdPasta.Metadata.ObjectID)
				require.Equal(t, resp.Metadata.Language, createdPasta.Metadata.Language)
				require.Equal(t, resp.Metadata.ID, createdPasta.Metadata.ID)
				require.Equal(t, resp.Metadata.Hash, createdPasta.Metadata.Hash)
				require.Equal(t, resp.Metadata.ID, createdPasta.Metadata.ID)

				require.NotEqual(t, resp.Message, createdPasta.Text)
				s3message, err := testApp.repos.S3.Get(t.Context(), resp.Metadata.ObjectID)
				require.NoError(t, err)
				require.Equal(t, s3message, "Message after update")

				elasticMessage, err := testApp.repos.Elastic.SearchWord("after")
				require.NoError(t, err)
				require.Equal(t, elasticMessage[0], resp.Metadata.ObjectID)
			},
		},
		{
			name:          "Pasta Not Found",
			withAuth:      true,
			userIDForAuth: 1,
			inputBody: dto.UpdateRequest{
				NewText: "Message after update",
			},
			expectedStatus: 404,
			checkResponse: func(t *testing.T, w *httptest.ResponseRecorder, resp *dto.SuccessUpdatedPastaResponse, createdPasta *dto.TextsWithMetadata) {
				require.Contains(t, w.Body.String(), customerrors.ErrPastaNotFound.Error())
			},
		},
		{
			name:      "No Access",
			withPasta: true,
			testPastas: TestPastaWithUserID{
				Pasta: dto.RequestCreatePasta{
					Message: "Before update",
				},
				UserID: 1,
			},
			withAuth:      true,
			userIDForAuth: 2,
			inputBody: dto.UpdateRequest{
				NewText: "Message after update",
			},
			expectedStatus: 403,
			checkResponse: func(t *testing.T, w *httptest.ResponseRecorder, resp *dto.SuccessUpdatedPastaResponse, createdPasta *dto.TextsWithMetadata) {
				require.Contains(t, w.Body.String(), customerrors.ErrNoAccess.Error())
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Cleanup(func() {
				cleanupAll(t, testApp.Postgres, testApp.Redis, testApp.Minio, testApp.ElasticService, testApp.Elastic, testApp.Bucket, testApp.Index)
			})

			body, err := json.Marshal(tt.inputBody)
			require.NoError(t, err)

			ctx := t.Context()

			pasta := &models.Pasta{}
			if tt.withPasta {
				var err error
				pasta, err = testApp.Services.Pasta.Create(ctx, &tt.testPastas.Pasta, tt.testPastas.UserID)
				require.NoError(t, err)
			}

			url := "/update/" + pasta.Hash
			if pasta.Hash == "" {
				url += "nothing"
			}

			req := httptest.NewRequest(http.MethodPut, url, bytes.NewBuffer(body))
			req.Header.Set("Content-Type", "application/json")

			if tt.withAuth {
				token, err := utils.GenerateTestJWT(tt.userIDForAuth)
				require.NoError(t, err)
				req.Header.Set("Authorization", "Bearer "+token)
			}

			w := httptest.NewRecorder()
			testApp.Engine.ServeHTTP(w, req)

			var resp dto.SuccessUpdatedPastaResponse
			_ = json.Unmarshal(w.Body.Bytes(), &resp)

			require.Equal(t, tt.expectedStatus, w.Code)
			tt.checkResponse(t, w, &resp, &dto.TextsWithMetadata{Text: tt.testPastas.Pasta.Message, Metadata: pasta})
		})
	}
}

func TestCreateFavorite(t *testing.T) {
	tests := []struct {
		name           string
		withPasta      bool
		testPasta      TestPastaWithUserID
		withAuth       bool
		userForAuth    int
		expectedStatus int
		query          string
		checkResponse  func(t *testing.T, w *httptest.ResponseRecorder, metadata *models.Pasta)
	}{
		{
			name:      "Success",
			withPasta: true,
			testPasta: TestPastaWithUserID{
				Pasta: dto.RequestCreatePasta{
					Message: "First message",
				},
				UserID: 1,
			},
			withAuth:       true,
			userForAuth:    1,
			expectedStatus: 200,
			checkResponse: func(t *testing.T, w *httptest.ResponseRecorder, metadata *models.Pasta) {
				require.Contains(t, w.Body.String(), "Successfully created")

				hash, err := testApp.repos.Database.Pasta().GetFavoriteAndCheckUser(t.Context(), 1, 1)
				require.NoError(t, err)

				require.Equal(t, metadata.Hash, hash)
			},
		},
		{
			name:      "Non Allowed",
			withPasta: true,
			testPasta: TestPastaWithUserID{
				Pasta: dto.RequestCreatePasta{
					Message:    "First message",
					Visibility: "private",
				},
				UserID: 1,
			},
			withAuth:       true,
			userForAuth:    1,
			expectedStatus: 400,
			checkResponse: func(t *testing.T, w *httptest.ResponseRecorder, metadata *models.Pasta) {
				require.Contains(t, w.Body.String(), customerrors.ErrNotAllowed.Error())
			},
		},
		{
			name:           "Pasta Not Found",
			withAuth:       true,
			userForAuth:    1,
			expectedStatus: 404,
			checkResponse: func(t *testing.T, w *httptest.ResponseRecorder, metadata *models.Pasta) {
				require.Contains(t, w.Body.String(), customerrors.ErrPastaNotFound.Error())
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Cleanup(func() {
				cleanupAll(t, testApp.Postgres, testApp.Redis, testApp.Minio, testApp.ElasticService, testApp.Elastic, testApp.Bucket, testApp.Index)
			})

			ctx := t.Context()

			pasta := &models.Pasta{}
			if tt.withPasta {
				var err error
				pasta, err = testApp.Services.Pasta.Create(ctx, &tt.testPasta.Pasta, tt.testPasta.UserID)
				require.NoError(t, err)

			} else {
				pasta.Hash = "nothing"
			}

			url := "/favorite/create/" + pasta.Hash
			if tt.query != "" {
				url += tt.query
			}

			req := httptest.NewRequest(http.MethodPost, url, nil)
			req.Header.Set("Content-Type", "application/json")

			if tt.withAuth {
				token, err := utils.GenerateTestJWT(tt.userForAuth)
				require.NoError(t, err)
				req.Header.Set("Authorization", "Bearer "+token)

				require.NoError(t, testUser(ctx, tt.testPasta.UserID))
			}

			w := httptest.NewRecorder()
			testApp.Engine.ServeHTTP(w, req)

			require.Equal(t, tt.expectedStatus, tt.expectedStatus)
			tt.checkResponse(t, w, pasta)
		})
	}
}

type testGetFavorite struct {
	Pasta      dto.RequestCreatePasta
	UserID     int
	FavoriteID int
}

func TestGetFavorite(t *testing.T) {
	tests := []struct {
		name          string
		withPasta     bool
		testPasta     testGetFavorite
		withAuth      bool
		userForAuth   int
		expectedCode  int
		query         string
		checkResponse func(t *testing.T, w *httptest.ResponseRecorder, resp *dto.GetPastaResponse, createdPasta *dto.TextsWithMetadata)
	}{
		{
			name:      "Success",
			withPasta: true,
			testPasta: testGetFavorite{
				Pasta: dto.RequestCreatePasta{
					Message: "Test message",
				},
				UserID:     1,
				FavoriteID: 1,
			},
			withAuth:     true,
			userForAuth:  1,
			expectedCode: 200,
			checkResponse: func(t *testing.T, w *httptest.ResponseRecorder, resp *dto.GetPastaResponse, createdPasta *dto.TextsWithMetadata) {
				require.Equal(t, resp.Text, createdPasta.Text)
				require.Empty(t, resp.Metadata)
			},
		},
		{
			name:      "Success with metadata",
			withPasta: true,
			testPasta: testGetFavorite{
				Pasta: dto.RequestCreatePasta{
					Message: "Test message",
				},
				UserID:     1,
				FavoriteID: 1,
			},
			withAuth:     true,
			userForAuth:  1,
			expectedCode: 200,
			query:        "?metadata=true",
			checkResponse: func(t *testing.T, w *httptest.ResponseRecorder, resp *dto.GetPastaResponse, createdPasta *dto.TextsWithMetadata) {
				require.Equal(t, resp.Text, createdPasta.Text)
				require.NotEmpty(t, resp.Metadata)
			},
		},
		{
			name:         "Pasta Not Found",
			withAuth:     true,
			userForAuth:  1,
			expectedCode: 400,
			checkResponse: func(t *testing.T, w *httptest.ResponseRecorder, resp *dto.GetPastaResponse, createdPasta *dto.TextsWithMetadata) {
				require.Contains(t, w.Body.String(), customerrors.ErrNotAllowed.Error())
			},
		},
		{
			name:      "Not Allowed",
			withPasta: true,
			testPasta: testGetFavorite{
				Pasta: dto.RequestCreatePasta{
					Message: "Test message",
				},
				UserID:     1,
				FavoriteID: 1,
			},
			withAuth:     true,
			userForAuth:  2,
			expectedCode: 400,
			checkResponse: func(t *testing.T, w *httptest.ResponseRecorder, resp *dto.GetPastaResponse, createdPasta *dto.TextsWithMetadata) {
				require.Contains(t, w.Body.String(), customerrors.ErrNotAllowed.Error())
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Cleanup(func() {
				cleanupAll(t, testApp.Postgres, testApp.Redis, testApp.Minio, testApp.ElasticService, testApp.Elastic, testApp.Bucket, testApp.Index)
			})

			ctx := t.Context()

			pasta := &models.Pasta{}
			if tt.withPasta {
				var err error
				pasta, err = testApp.Services.Pasta.Create(ctx, &tt.testPasta.Pasta, tt.testPasta.UserID)
				require.NoError(t, err)

				require.NoError(t, testUser(ctx, tt.testPasta.UserID))

				require.NoError(t, testApp.repos.Database.Pasta().Favorite(ctx, pasta.Hash, tt.testPasta.UserID))
			}

			url := fmt.Sprintf("/favorite/%d", tt.testPasta.FavoriteID)
			if tt.query != "" {
				url += tt.query
			}

			req := httptest.NewRequest(http.MethodGet, url, nil)

			if tt.withAuth {
				token, err := utils.GenerateTestJWT(tt.userForAuth)
				require.NoError(t, err)
				req.Header.Set("Authorization", "Bearer "+token)
			}

			w := httptest.NewRecorder()
			testApp.Engine.ServeHTTP(w, req)

			resp := &dto.GetPastaResponse{}
			_ = json.Unmarshal(w.Body.Bytes(), &resp)

			require.Equal(t, tt.expectedCode, w.Code)
			tt.checkResponse(t, w, resp, &dto.TextsWithMetadata{
				Text:     tt.testPasta.Pasta.Message,
				Metadata: pasta,
			})
		})
	}
}

func TestDeleteFavorite(t *testing.T) {
	tests := []struct {
		name          string
		withPasta     bool
		testPasta     testGetFavorite
		withAuth      bool
		userForAuth   int
		expectedCode  int
		checkResponse func(t *testing.T, w *httptest.ResponseRecorder, createdPasta *dto.TextsWithMetadata)
	}{
		{
			name:      "Success",
			withPasta: true,
			testPasta: testGetFavorite{
				Pasta: dto.RequestCreatePasta{
					Message: "Message",
				},
				UserID:     1,
				FavoriteID: 1,
			},
			withAuth:     true,
			userForAuth:  1,
			expectedCode: 200,
			checkResponse: func(t *testing.T, w *httptest.ResponseRecorder, createdPasta *dto.TextsWithMetadata) {
				require.Contains(t, w.Body.String(), "Successfully deleted")

				exists, err := isFavoriteExists(t.Context())
				require.NoError(t, err)
				require.False(t, exists)
			},
		},
		{
			name:         "Pasta Not Found",
			withAuth:     true,
			userForAuth:  1,
			expectedCode: 400,
			checkResponse: func(t *testing.T, w *httptest.ResponseRecorder, createdPasta *dto.TextsWithMetadata) {
				require.Contains(t, w.Body.String(), customerrors.ErrNotAllowed.Error())
			},
		},
		{
			name:      "Wrong User",
			withPasta: true,
			testPasta: testGetFavorite{
				Pasta: dto.RequestCreatePasta{
					Message: "Message",
				},
				UserID:     1,
				FavoriteID: 1,
			},
			withAuth:     true,
			userForAuth:  2,
			expectedCode: 400,
			checkResponse: func(t *testing.T, w *httptest.ResponseRecorder, createdPasta *dto.TextsWithMetadata) {
				require.Contains(t, w.Body.String(), customerrors.ErrNotAllowed.Error())
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Cleanup(func() {
				cleanupAll(t, testApp.Postgres, testApp.Redis, testApp.Minio, testApp.ElasticService, testApp.Elastic, testApp.Bucket, testApp.Index)
			})

			ctx := t.Context()

			pasta := &models.Pasta{}
			if tt.withPasta {
				var err error
				pasta, err = testApp.Services.Pasta.Create(ctx, &tt.testPasta.Pasta, tt.testPasta.UserID)
				require.NoError(t, err)

				require.NoError(t, testUser(ctx, tt.testPasta.UserID))

				require.NoError(t, testApp.repos.Database.Pasta().Favorite(ctx, pasta.Hash, tt.testPasta.UserID))
			}

			url := fmt.Sprintf("/favorite/%d", tt.testPasta.FavoriteID)

			req := httptest.NewRequest(http.MethodDelete, url, nil)

			if tt.withAuth {
				token, err := utils.GenerateTestJWT(tt.userForAuth)
				require.NoError(t, err)
				req.Header.Set("Authorization", "Bearer "+token)
			}

			w := httptest.NewRecorder()
			testApp.Engine.ServeHTTP(w, req)

			resp := &dto.GetPastaResponse{}
			_ = json.Unmarshal(w.Body.Bytes(), &resp)

			require.Equal(t, tt.expectedCode, w.Code)
			tt.checkResponse(t, w, &dto.TextsWithMetadata{
				Text:     tt.testPasta.Pasta.Message,
				Metadata: pasta,
			})
		})
	}
}

func TestPaginateFavorite(t *testing.T) {
	tests := []struct {
		name           string
		withPastas     bool
		testPastas     []testGetFavorite
		users          int
		favoriteCreate func(t *testing.T, forCreate map[string]string)
		withAuth       bool
		userForAuth    int
		expectedCode   int
		query          string
		checkResponse  func(t *testing.T, w *httptest.ResponseRecorder, resp *dto.PaginatedPastaDTO)
	}{
		{
			name:       "Success",
			withPastas: true,
			testPastas: []testGetFavorite{
				{
					Pasta: dto.RequestCreatePasta{
						Message: "First Message",
					},
					UserID: 1,
				},
				{
					Pasta: dto.RequestCreatePasta{
						Message: "Second Message",
					},
					UserID: 1,
				},
			},
			users: 2,
			favoriteCreate: func(t *testing.T, forCreate map[string]string) {
				require.NoError(t, testApp.repos.Database.Pasta().Favorite(t.Context(), forCreate["First Message"], 2))
			},
			withAuth:     true,
			userForAuth:  2,
			expectedCode: 200,
			query:        "?limit=10&page=1",
			checkResponse: func(t *testing.T, w *httptest.ResponseRecorder, resp *dto.PaginatedPastaDTO) {
				require.Len(t, resp.Pastas, 1)
				require.Equal(t, 1, resp.Total)
				require.Equal(t, 10, resp.Limit)
				require.Equal(t, 1, resp.Page)

			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Cleanup(func() {
				cleanupAll(t, testApp.Postgres, testApp.Redis, testApp.Minio, testApp.ElasticService, testApp.Elastic, testApp.Bucket, testApp.Index)
			})

			ctx := t.Context()

			if tt.withPastas {
				for i := 1; i <= tt.users; i++ {
					require.NoError(t, testUser(ctx, i))
				}

				forCreateMap := map[string]string{}

				for _, pasta := range tt.testPastas {
					p, err := testApp.Services.Pasta.Create(ctx, &pasta.Pasta, pasta.UserID)
					require.NoError(t, err)

					forCreateMap[pasta.Pasta.Message] = p.Hash
				}

				tt.favoriteCreate(t, forCreateMap)
			}

			url := "/favorite/paginate"
			if tt.query != "" {
				url += tt.query
			}

			req := httptest.NewRequest(http.MethodGet, url, nil)

			if tt.withAuth {
				token, err := utils.GenerateTestJWT(tt.userForAuth)
				require.NoError(t, err)
				req.Header.Set("Authorization", "Bearer "+token)
			}

			w := httptest.NewRecorder()
			testApp.Engine.ServeHTTP(w, req)

			resp := &dto.PaginatedPastaDTO{}
			_ = json.Unmarshal(w.Body.Bytes(), &resp)

			require.Equal(t, tt.expectedCode, w.Code, w.Body.String())
			tt.checkResponse(t, w, resp)
		})
	}
}

func isFavoriteExists(ctx context.Context) (bool, error) {
	exists := false
	err := testApp.Postgres.GetContext(ctx, &exists, "SELECT EXISTS(SELECT 1 FROM favorites)")
	return exists, err
}

func testUser(ctx context.Context, userID int) error {
	_, err := testApp.Postgres.ExecContext(ctx, "INSERT INTO users (name, email, password_hash) VALUES($1, $2, $3)", fmt.Sprintf("test%d", userID), fmt.Sprintf("test%d", userID), fmt.Sprintf("test%d", userID))
	return err
}
