package elastic

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"pastebin/internal/config"
	"time"

	"github.com/elastic/go-elasticsearch/v8"
	"github.com/theartofdevel/logging"
)

type Logger struct {
	ctx context.Context
}

func (l *Logger) LogRoundTrip(req *http.Request, res *http.Response, err error,
	start time.Time, dur time.Duration) error {
	logging.WithAttrs(l.ctx,
		logging.StringAttr("Method", req.Method),
		logging.StringAttr("URL", req.URL.String()),
		logging.StringAttr("Status", res.Status),
		logging.StringAttr("Duration", dur.String()),
	).Debug("New HTTP elasticsearch request")
	return nil
}

func (l *Logger) RequestBodyEnabled() bool  { return false } // если true, то клиент будет передавать копию тема запроса в логгер
func (l *Logger) ResponseBodyEnabled() bool { return false } // тоже самое, только при ответе

type ElasticClient struct {
	client *elasticsearch.Client
	ctx    context.Context
}

func NewElasticClient(ctx context.Context, cfg config.ElasticConfig) (*ElasticClient, error) {
	addresses := []string{}

	if cfg.Mode == "cluster" {
		addresses = cfg.Addrs
	} else {
		addresses = append(addresses, cfg.Addr)
	}

	esCfg := elasticsearch.Config{
		Addresses:           addresses,
		Username:            cfg.Username,
		Password:            cfg.Password,
		Logger:              &Logger{ctx: ctx},
		RetryOnStatus:       cfg.RetryOnStatus,
		MaxRetries:          cfg.MaxRetries,
		CompressRequestBody: cfg.CompressRequstBody, // сжиимает тело запроса (gzip)
	}

	client, err := elasticsearch.NewClient(esCfg)
	if err != nil {
		return nil, err
	}
	return &ElasticClient{
		client: client,
		ctx:    ctx,
	}, nil
}

func (e *ElasticClient) CreateSearchIndex(index string) (string, error) {
	formattedIndexName := index + "-" + time.Now().Format("2006.01.02")

	existsRes, err := e.client.Indices.Exists([]string{formattedIndexName},
		e.client.Indices.Exists.WithContext(e.ctx),
	)
	if err != nil {
		return "", fmt.Errorf("failed to check if index exists: %w", err)
	}
	defer existsRes.Body.Close()

	if existsRes.StatusCode == http.StatusOK {
		logging.L(e.ctx).Debug(fmt.Sprintf("Index [%s] already exists", formattedIndexName))
		return formattedIndexName, nil
	}

	mapping := map[string]interface{}{
		"mappings": map[string]interface{}{
			"properties": map[string]interface{}{
				"content": map[string]interface{}{
					"type":     "text",
					"analyzer": "russian",
				},
				"tags": map[string]interface{}{
					"type": "keyword",
				},
			},
		},
	}

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(mapping); err != nil {
		return "", fmt.Errorf("failed to encode mapping: %w", err)
	}

	createRes, err := e.client.Indices.Create(
		formattedIndexName,
		e.client.Indices.Create.WithBody(&buf),
		e.client.Indices.Create.WithContext(e.ctx),
	)

	if err != nil {
		return "", fmt.Errorf("failed to create index: %w", err)
	}
	defer createRes.Body.Close()

	if createRes.IsError() {
		return "", fmt.Errorf("index creation failed: %s", createRes)
	}

	logging.L(e.ctx).Debug(fmt.Sprintf("Index [%s] created successfully", formattedIndexName))
	return formattedIndexName, nil
}

func (e *ElasticClient) Close() {
	if e.client != nil && e.client.Transport != nil {
		if transport, ok := e.client.Transport.(interface{ CloseIdleConnections() }); ok {
			transport.CloseIdleConnections()
		}
	}
	e.client = nil
}

func (e *ElasticClient) Client() *elasticsearch.Client {
	return e.client
}
