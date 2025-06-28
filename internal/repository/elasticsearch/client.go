package elasticsearch

import (
	"bytes"
	"encoding/json"
	"fmt"
	"pastebin/internal/config"

	"github.com/elastic/go-elasticsearch/v8"
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

func (e *ElasticClient) EnsureIndex(index string) error {
	res, err := e.client.Indices.Exists([]string{index})
	if err != nil {
		return err
	}
	defer res.Body.Close()

	if res.StatusCode == 200 {
		return nil
	}

	// Создаем с маппингом (настраивается по нужным полям)
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
		return err
	}

	createRes, err := e.client.Indices.Create(index, e.client.Indices.Create.WithBody(&buf))
	if err != nil {
		return err
	}
	defer createRes.Body.Close()

	if createRes.IsError() {
		return fmt.Errorf("ошибка создания индекса: %s", createRes.String())
	}

	return nil
}

func (e *ElasticClient) Client() *elasticsearch.Client {
	return e.client
}

func byteToString(text *[]byte) *string {
	a := string(*text)
	return &a
}
