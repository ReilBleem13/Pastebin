package elasticsearch

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"pastebin/internal/domain"
	"time"

	"github.com/elastic/go-elasticsearch/v8"
)

type Elastic struct {
	client *elasticsearch.Client
}

func NewElastic(client *elasticsearch.Client) domain.Elastic {
	return &Elastic{client: client}
}

func (e *Elastic) NewIndex(text *[]byte, key, index string, tags []string) error {
	start := time.Now()
	if text == nil {
		return fmt.Errorf("empty")
	}

	doc := map[string]interface{}{
		"content": *byteToString(text),
		"tags":    tags,
	}

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(doc); err != nil {
		return err
	}

	res, err := e.client.Index(index, &buf, e.client.Index.WithDocumentID(key), e.client.Index.WithRefresh("true"))
	if err != nil {
		return err
	}
	defer res.Body.Close()

	if res.IsError() {
		return fmt.Errorf("ошибка индексации: %s", res.String())
	}
	log.Printf("Новый индекс. Время выполнения: %v", time.Since(start).Seconds())
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
