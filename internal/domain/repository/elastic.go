package domain

//go:generate mockgen -source=elastic.go -destination=../mocks/repository/elastic.go -package=mocks

type Elastic interface {
	NewIndex(text []byte, key, index string, tags []string) error
	SearchWord(word, index string) ([]string, error)
}
