package elasticsearch

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	domain "pastebin/internal/domain/repository"
	"time"

	"github.com/elastic/go-elasticsearch/v8"
)

type Elastic struct {
	client *elasticsearch.Client
}

func NewElastic(client *elasticsearch.Client) domain.Elastic {
	return &Elastic{client: client}
}

type PastaDocument struct {
	Content string   `json:"content"`
	Tags    []string `json:"tags,omitempty"`
}

func (e *Elastic) NewIndex(text []byte, objectID, index string, tags []string) error {
	doc := PastaDocument{
		Content: string(text),
		Tags:    tags,
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

func (e *Elastic) SearchWord(word, index string) ([]string, error) {
	start := time.Now()

	if word == "" {
		log.Println("Пустой поисковый запрос — ничего не ищем.")
		return nil, nil
	}

	query := map[string]interface{}{
		"query": map[string]interface{}{
			"match": map[string]interface{}{
				"content": word,
			},
		},
	}

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(query); err != nil {
		return nil, err
	}

	res, err := e.client.Search(
		e.client.Search.WithIndex(index),
		e.client.Search.WithBody(&buf),
		e.client.Search.WithTrackTotalHits(true),
	)

	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	if res.IsError() {
		return nil, fmt.Errorf("ошибка от Elasticsearch: %s", res.String())
	}

	var r map[string]interface{}
	if err := json.NewDecoder(res.Body).Decode(&r); err != nil {
		return nil, err
	}

	hitsWrapper, ok := r["hits"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("не найдено поле hits в ответе Elasticsearch")
	}

	hits, ok := hitsWrapper["hits"].([]interface{})
	if !ok {
		return nil, fmt.Errorf("не найдено поле hits.hits в ответе Elasticsearch")
	}

	var filenames []string
	for _, h := range hits {
		hit, ok := h.(map[string]interface{})
		if !ok {
			continue
		}

		id, ok := hit["_id"].(string)
		if ok {
			filenames = append(filenames, id)
		}
	}

	log.Printf("Поиск завершен за %.3f сек.", time.Since(start).Seconds())
	return filenames, nil
}
