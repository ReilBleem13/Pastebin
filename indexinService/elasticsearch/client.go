package elastic

import (
	"bytes"
	"encoding/json"
	"fmt"

	"github.com/elastic/go-elasticsearch/v8"
)

type Elastic interface {
	NewIndex(text []byte, objectID, index string) error
}

type ElasticClient struct {
	client *elasticsearch.Client
}

type PastaDocument struct {
	Content string `json:"content"`
}

func NewElasticClient(address []string) (*ElasticClient, error) {
	client, err := elasticsearch.NewClient(elasticsearch.Config{
		Addresses: address,
	})
	if err != nil {
		return nil, err
	}
	return &ElasticClient{client: client}, nil
}
func (e *ElasticClient) NewIndex(text []byte, objectID, index string) error {
	doc := PastaDocument{
		Content: string(text),
	}

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(doc); err != nil {
		return fmt.Errorf("failed to encode document: %w", err)
	}

	// флаг true говорит эластику немедленно обновить индекс (высокая нагрузка) (e.client.Index.WithRefresh("true")))
	res, err := e.client.Index(
		index,
		&buf,
		e.client.Index.WithDocumentID(objectID))
	if err != nil {
		return fmt.Errorf("indexing request failed: %w", err)
	}
	defer res.Body.Close()

	if res.IsError() {
		var errMap map[string]interface{}
		if err := json.NewDecoder(res.Body).Decode(&errMap); err != nil {
			return fmt.Errorf("indexing error, status: %s", res.Status())
		}
		return fmt.Errorf("indexing error, status: %s, response: %v", res.Status(), errMap["error"])
	}
	return nil
}

func (e *ElasticClient) Client() *elasticsearch.Client {
	return e.client
}
