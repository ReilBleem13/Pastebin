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
	"pastebin/internal/handler"
	elasticClient "pastebin/internal/infrastructure/elastic"
	minioClient "pastebin/internal/infrastructure/minio"
	postgresClient "pastebin/internal/infrastructure/postgres"
	redisClient "pastebin/internal/infrastructure/redis"
	"pastebin/internal/models"
	"pastebin/internal/repository"
	"pastebin/internal/service"
	"pastebin/internal/utils"
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
	Engine   *gin.Engine
	Postgres *sqlx.DB
	Redis    *redis.Client
	Minio    *minio.Client
	Elastic  *elasticsearch.Client
	Bucket   string
	Index    string
	Services *service.Service
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
			Image:        "docker.elastic.co/elasticsearch/elasticsearch:8.9.0",
			Env:          map[string]string{"discovery.type": "single-node", "xpack.security.enabled": "false"},
			ExposedPorts: []string{"9200/tcp"},
			WaitingFor:   wait.ForListeningPort("9200/tcp").WithStartupTimeout(60 * time.Second),
		},
		Started: true,
	})
	if err != nil {
		return nil, err
	}

	elasticPort, _ := elasticC.MappedPort(ctx, "9200")
	elasticHost, _ := elasticC.Host(ctx)
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

	postgres, err := postgresClient.NewPostgresDB(ctx, env.PostgresURI)
	if err != nil {
		return nil, err
	}

	redis, err := redisClient.NewRedisClient(ctx, config.RedisConfig{Host: env.RedisAddr})
	if err != nil {
		return nil, err
	}

	minio, err := minioClient.NewMinioClient(ctx, config.MinioConfig{
		Host:     env.MinioEndpoint,
		Bucket:   env.MinioBucket,
		Rootuser: env.MinioRootUser,
		Password: env.MinioPassword,
		Ssl:      false,
	}, 10)
	if err != nil {
		return nil, err
	}

	elastiCli, err := elasticClient.NewElasticClient(config.ElasticConfig{
		Addresses: env.ElasticURL,
		Index:     env.ElasticIndex,
	})
	if err != nil {
		return nil, err
	}

	err = runMigrations(env.MigrateURI, "../../../migrations")
	if err != nil {
		return nil, err
	}

	repos := repository.NewRepository(postgres.Client(), redis.Client(), minio.Client(),
		elastiCli.Client(), minio.Pool(), env.MinioBucket)
	logger := logging.GetLogger()
	services := service.NewService(repos, nil, logger) // KAFKA УКАЗАНА КАК NIL. CURSOR! KAFKA УКАЗАНА КАК NIL!!!
	h := handler.NewHandler(services, logger)

	r := gin.Default()
	auth := r.Group("/")
	auth.Use(h.AuthMiddleWare())
	{
		auth.POST("/create", h.AccessPostMiddleware(), h.CreatePastaHandler)
		auth.GET("/receive/:objectID", h.AccessByKeyMiddleware(), h.GetPastaHandler)
		auth.DELETE("/delete/:objectID", h.AccessByKeyMiddleware(), h.DeletePastaHandler)
	}
	return &TestApp{
		Engine:   r,
		Postgres: postgres.Client(),
		Redis:    redis.Client(),
		Minio:    minio.Client(),
		Elastic:  elastiCli.Client(),
		Bucket:   "testchukki",
		Index:    "testindex",
		Services: services,
	}, nil
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

	_, err = elastic.Indices.Delete([]string{elasticIndex})
	require.NoError(t, err)
}

func isInDatabase(objectID string, client *sqlx.DB) (error, bool) {
	var exists bool
	err := client.Get(&exists, "SELECT EXISTS(SELECT 1 FROM pastas WHERE object_id = $1)", objectID)
	return err, exists
}

func isInS3(ctx context.Context, objectID, bucket string, client *minio.Client) error {
	_, err := client.StatObject(ctx, bucket, objectID, minio.GetObjectOptions{})
	return err
}

func isTextInCache(ctx context.Context, hash string, client *redis.Client) error {
	return client.Get(ctx, fmt.Sprintf("text:%s", hash)).Err()
}

func isViewsInCache(ctx context.Context, hash string, client *redis.Client) error {
	return client.Get(ctx, fmt.Sprintf("views:%s", hash)).Err()
}

func TestMain(m *testing.M) {
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
				require.Equal(t, "public", resp.Metadata.Visibility)
				require.NotEmpty(t, resp.Link)
				require.NotEmpty(t, resp.Metadata)

				hash := strings.TrimPrefix(resp.Link, "http://localhost:8080/receive/")
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
				require.Equal(t, "public", resp.Metadata.Visibility)
				require.Equal(t, resp.Metadata.CreatedAt.Add(7*24*time.Hour), resp.Metadata.ExpiresAt)
				require.NotEmpty(t, resp.Link)
				require.NotEmpty(t, resp.Metadata)

				hash := strings.TrimPrefix(resp.Link, "http://localhost:8080/receive/")
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
				require.Equal(t, "private", resp.Metadata.Visibility)
				require.Equal(t, resp.Metadata.CreatedAt.Add(7*24*time.Hour), resp.Metadata.ExpiresAt)
				require.NotEmpty(t, resp.Link)
				require.NotEmpty(t, resp.Metadata)

				log.Printf("objectID: %s", resp.Metadata.Key)

				hash := strings.TrimPrefix(resp.Link, "http://localhost:8080/receive/")
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
				Visibility: "Private",
			},
			expectedStatus: 401,
			withAuth:       false,
			checkResponse: func(t *testing.T, resp *dto.SuccessCreatePastaResponse, w *httptest.ResponseRecorder, app *TestApp) {
				require.Contains(t, w.Body.String(), "unathorized: create private pastas require login")
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
				cleanupAll(t, testApp.Postgres, testApp.Redis, testApp.Minio, testApp.Elastic, testApp.Bucket, testApp.Index)
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
		checkResponse  func(t *testing.T, resp *dto.GetPastaResponse, w *httptest.ResponseRecorder)
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
			checkResponse: func(t *testing.T, resp *dto.GetPastaResponse, w *httptest.ResponseRecorder) {
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
			checkResponse: func(t *testing.T, resp *dto.GetPastaResponse, w *httptest.ResponseRecorder) {
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
			checkResponse: func(t *testing.T, resp *dto.GetPastaResponse, w *httptest.ResponseRecorder) {
				require.Equal(t, "Successfully got pasta", resp.Message)
				require.Equal(t, "Test Message #1", resp.Text)

				require.Equal(t, 0, resp.Metadata.UserID)
				require.Equal(t, "go", resp.Metadata.Language)
				require.Equal(t, "public", resp.Metadata.Visibility)
				require.Equal(t, 2, resp.Metadata.Views)
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
			checkResponse: func(t *testing.T, resp *dto.GetPastaResponse, w *httptest.ResponseRecorder) {
				require.Equal(t, "Successfully got pasta", resp.Message)
				require.Equal(t, "Test Message #1", resp.Text)

				require.Equal(t, 0, resp.Metadata.UserID)
				require.Equal(t, "go", resp.Metadata.Language)
				require.Equal(t, "public", resp.Metadata.Visibility)
				require.Equal(t, 2, resp.Metadata.Views)
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
			checkResponse: func(t *testing.T, resp *dto.GetPastaResponse, w *httptest.ResponseRecorder) {
				require.Equal(t, "Successfully got pasta", resp.Message)
				require.Equal(t, "Test Message #1", resp.Text)
				require.Equal(t, 1, resp.Metadata.UserID)
				require.Equal(t, "go", resp.Metadata.Language)
				require.Equal(t, "public", resp.Metadata.Visibility)
				require.Equal(t, 2, resp.Metadata.Views)
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
			checkResponse: func(t *testing.T, resp *dto.GetPastaResponse, w *httptest.ResponseRecorder) {
				require.Equal(t, "Successfully got pasta", resp.Message)
				require.Equal(t, "Test Message #1", resp.Text)
				require.Equal(t, 1, resp.Metadata.UserID)
				require.Equal(t, "go", resp.Metadata.Language)
				require.Equal(t, "private", resp.Metadata.Visibility)
				require.Equal(t, 2, resp.Metadata.Views)
				require.Equal(t, resp.Metadata.CreatedAt.Add(24*time.Hour), resp.Metadata.ExpiresAt)
			},
		},
		{
			name: "Pasta Not Found",
			url: func(urlRrefix string, _ string) string {
				return urlRrefix + "/" + "notfoundpasta"
			},
			expectedStatus: 404,
			checkResponse: func(t *testing.T, resp *dto.GetPastaResponse, w *httptest.ResponseRecorder) {
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
			checkResponse: func(t *testing.T, resp *dto.GetPastaResponse, w *httptest.ResponseRecorder) {
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
			checkResponse: func(t *testing.T, resp *dto.GetPastaResponse, w *httptest.ResponseRecorder) {
				require.Contains(t, w.Body.String(), "no access, private pasta")
			},
		},
		{
			name:      "Unauthorized",
			withPasta: true,
			testPasta: &dto.RequestCreatePasta{
				Message:    "Test Message #1",
				Visibility: "Private",
			},
			url: func(urlRrefix string, hash string) string {
				return urlRrefix + "/" + hash
			},
			expectedStatus: 401,
			checkResponse: func(t *testing.T, resp *dto.GetPastaResponse, w *httptest.ResponseRecorder) {
				require.Contains(t, w.Body.String(), "unauthorized: private pasta")
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
			checkResponse: func(t *testing.T, resp *dto.GetPastaResponse, w *httptest.ResponseRecorder) {
				require.Contains(t, w.Body.String(), "invalid 'metadata' query parameter")
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
			checkResponse: func(t *testing.T, resp *dto.GetPastaResponse, w *httptest.ResponseRecorder) {
				require.Contains(t, w.Body.String(), "invalid character")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Cleanup(func() {
				cleanupAll(t, testApp.Postgres, testApp.Redis, testApp.Minio, testApp.Elastic, testApp.Bucket, testApp.Index)
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
			tt.checkResponse(t, &resp, w)
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
				require.Contains(t, w.Body.String(), "deleted")

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
				require.Contains(t, w.Body.String(), "deleted")

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
				require.Contains(t, w.Body.String(), "pasta is not found")
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
				require.Contains(t, w.Body.String(), "unauthorized: private pasta")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Cleanup(func() {
				cleanupAll(t, testApp.Postgres, testApp.Redis, testApp.Minio, testApp.Elastic, testApp.Bucket, testApp.Index)
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
