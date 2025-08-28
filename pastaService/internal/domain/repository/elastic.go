package domain

import "context"

//go:generate mockgen -source=elastic.go -destination=../mocks/repository/elastic.go -package=mocks

type Elastic interface {
	SearchWord(word string) ([]string, error)
	DeleteDocument(ctx context.Context, objectID string) error
	Indexing(text []byte, objectID string) error

	DeleteManyDocument(ctx context.Context, objectIDs []string) error
}
