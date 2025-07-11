package elastic

import (
	"bytes"
	"context"
	"fmt"

	"github.com/elastic/go-elasticsearch/esapi"
	"github.com/elastic/go-elasticsearch/v8"
)

type Elastic interface {
	DeleteDocuments(ctx context.Context, objectIDs []string, index string) error
}

type ElasticClient struct {
	client *elasticsearch.Client
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

func (e *ElasticClient) DeleteDocuments(ctx context.Context, objectIDs []string, index string) error {
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

func (e *ElasticClient) Client() *elasticsearch.Client {
	return e.client
}
