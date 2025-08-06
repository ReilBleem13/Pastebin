package elastic

import (
	"bytes"
	"cleanService/config"
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/elastic/go-elasticsearch/esapi"
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

type Elastic interface {
	DeleteDocuments(ctx context.Context, objectIDs []string, index string) error
}

type ElasticClient struct {
	client *elasticsearch.Client
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
	}, nil
}

func (e *ElasticClient) DeleteDocuments(ctx context.Context, objectIDs []string, index string) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}

	var buf bytes.Buffer
	for _, id := range objectIDs {
		meta := fmt.Sprintf(`{ "delete" : { "_index" : "%s", "_id" : "%s" } }`, index, id)
		buf.WriteString(meta + "\n")
	}

	req := esapi.BulkRequest{
		Body: &buf,
	}

	res, err := req.Do(ctx, e.client)
	if err != nil {
		return fmt.Errorf("bulk delete request failed: %w", err)
	}
	defer res.Body.Close()

	if res.IsError() {
		return fmt.Errorf("bulk delete error: %s", res.String())
	}
	return nil
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
