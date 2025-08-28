package elasticsearch

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	domain "pastebin/internal/domain/repository"
	customerrors "pastebin/internal/errors"

	"github.com/elastic/go-elasticsearch/v8"
	"github.com/elastic/go-elasticsearch/v8/esapi"
)

type Elastic struct {
	client *elasticsearch.Client
	index  string
}

func NewElastic(client *elasticsearch.Client, index string) domain.Elastic {
	return &Elastic{
		client: client,
		index:  index,
	}
}

type PastaDocument struct {
	Content string   `json:"content"`
	Tags    []string `json:"tags,omitempty"`
}

func (e *Elastic) Indexing(text []byte, objectID string) error {
	doc := PastaDocument{
		Content: string(text),
	}

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(doc); err != nil {
		return fmt.Errorf("failed to encode document: %w", err)
	}

	// флаг true говорит эластику немедленно обновить индекс (высокая нагрузка) (e.client.Index.WithRefresh("true")))

	res, err := e.client.Index(
		e.index,
		&buf,
		e.client.Index.WithDocumentID(objectID),
		e.client.Index.WithRefresh("wait_for"),
	)
	if err != nil {
		return fmt.Errorf("indexing request failed: %w", err)
	}
	defer res.Body.Close()

	// log.Printf("Json Format: %v\n", res)
	// Json Format: [201 Created] {"_index":"pastas","_id":"public:bf67664f-59c8-455c-a48b-92bc0265ae22.txt","_version":1,"result":"created","_shards":{"total":2,"successful":1,"failed":0},"_seq_no":7,"_primary_term":1}

	if res.IsError() {
		var errMap map[string]interface{}
		if err := json.NewDecoder(res.Body).Decode(&errMap); err != nil {
			return fmt.Errorf("failed to decode errMap: %w", err)
		}
		return fmt.Errorf("indexing error, status: %s, response: %v", res.Status(), errMap["error"])
	}
	return nil
}

func (e *Elastic) DeleteDocument(ctx context.Context, objectID string) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}

	req := esapi.DeleteRequest{
		Index:      e.index,
		DocumentID: objectID,
	}

	res, err := req.Do(ctx, e.client)
	if err != nil {
		return fmt.Errorf("failed to create delete request: %w", err)
	}
	defer res.Body.Close()

	if res.IsError() {
		return fmt.Errorf("failed to delete request: %w", err)
	}
	return nil
}

func (e *Elastic) DeleteManyDocument(ctx context.Context, objectIDs []string) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}

	var buf bytes.Buffer
	for _, id := range objectIDs {
		meta := fmt.Sprintf(`{ "delete" : { "_index" : "%s", "_id" : "%s" } }`, e.index, id)
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

func (e *Elastic) SearchWord(word string) ([]string, error) {
	ctx := context.Background()

	if word == "" {
		log.Println("Пустой поисковый запрос — ничего не ищем.")
		return nil, customerrors.ErrEmptySearchField
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
		e.client.Search.WithContext(ctx),
		e.client.Search.WithIndex(e.index),
		e.client.Search.WithBody(&buf),
		e.client.Search.WithTrackTotalHits(true),
		e.client.Search.WithTerminateAfter(10000),
		e.client.Search.WithPretty(),
	)

	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	if res.IsError() {
		var e map[string]interface{}
		if err := json.NewDecoder(res.Body).Decode(&e); err != nil {
			return nil, fmt.Errorf("failed to decode error: %w", err)
		}
		return nil, fmt.Errorf("error searching for documents: %s", e)
	}

	var r map[string]interface{}
	if err := json.NewDecoder(res.Body).Decode(&r); err != nil {
		return nil, err
	}

	/*
		Result after decode:
		map[_shards:map[failed:0 skipped:0 successful:1 total:1]
		hits:map[
			hits:[
				map[_id:public:c982494a-83d6-4ab6-96cb-d0a3e6d6f19a.txt _index:pastas _score:1.9071718 _source:map[content:Один кот]]
				map[_id:public:a661cfd2-d25b-4060-a1d9-6dd91e5a7d1f.txt _index:pastas _score:1.9071718 _source:map[content:Два кота]]]
				max_score:1.9071718
			total:map[relation:eq value:2]]
			timed_out:false took:5]: %+v", hitsWrapper)
	*/

	hitsWrapper, ok := r["hits"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("не найдено поле hits в ответе Elasticsearch")
	}

	/*
		HITS-WRAPPER:
		map[
			hits:[
				map[_id:public:c982494a-83d6-4ab6-96cb-d0a3e6d6f19a.txt _index:pastas _score:1.9071718 _source:map[content:Один кот]]
				map[_id:public:a661cfd2-d25b-4060-a1d9-6dd91e5a7d1f.txt _index:pastas _score:1.9071718 _source:map[content:Два кота]]]
			max_score:1.9071718
			total:map[relation:eq value:2]]
	*/

	hits, ok := hitsWrapper["hits"].([]interface{})
	if !ok {
		return nil, fmt.Errorf("не найдено поле hits.hits в ответе Elasticsearch")
	}

	/*
		HITS:
		map[_id:public:c982494a-83d6-4ab6-96cb-d0a3e6d6f19a.txt _index:pastas _score:1.9071718 _source:map[content:Один кот]]
		map[_id:public:a661cfd2-d25b-4060-a1d9-6dd91e5a7d1f.txt _index:pastas _score:1.9071718 _source:map[content:Два кота]])
	*/

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
	log.Println(filenames)
	return filenames, nil
}
