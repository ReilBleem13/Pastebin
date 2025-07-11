package elastic

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"pastebin/internal/config"
	"time"

	"github.com/elastic/go-elasticsearch/v8"
	"github.com/elastic/go-elasticsearch/v8/esapi"
)

type ElasticClient struct {
	client *elasticsearch.Client
	index  string
}

func NewElasticClient(cfg config.ElasticConfig) (*ElasticClient, error) {
	client, err := elasticsearch.NewClient(elasticsearch.Config{
		Addresses: cfg.Addresses,
	})
	if err != nil {
		return nil, err
	}
	return &ElasticClient{client: client, index: cfg.Index}, nil
}

func (e *ElasticClient) CreateSearchIndex(index string) (string, error) {
	ctx := context.Background()
	formattedIndexName := index + "-" + time.Now().Format("2006.01.02")

	existReq := esapi.IndicesExistsRequest{
		Index: []string{formattedIndexName},
	}
	existsRes, err := existReq.Do(ctx, e.client)
	if err != nil {
		return "", err
	}
	defer existsRes.Body.Close()

	if existsRes.StatusCode == 200 {
		log.Println("Index - " + formattedIndexName + " already exists")
		return formattedIndexName, nil
	}

	mapping := map[string]interface{}{
		"mappings": map[string]interface{}{
			"properties": map[string]interface{}{
				"content": map[string]interface{}{
					"type":     "text",
					"analyzer": "russian",
				},
				"tags": map[string]interface{}{"type": "keyword"},
			},
		},
	}

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(mapping); err != nil {
		return "", fmt.Errorf("failed encode mapping: %s", err)
	}

	req := esapi.IndicesCreateRequest{
		Index: formattedIndexName,
		Body:  &buf,
	}

	res, err := req.Do(ctx, e.client)
	if err != nil {
		return "", err
	}
	defer res.Body.Close()

	if res.IsError() {
		return "", fmt.Errorf("error creating index: %s", res.Status())
	}

	log.Println("Index - " + formattedIndexName + " created successfully")
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
