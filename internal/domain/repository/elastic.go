package domain

//go:generate mockgen -source=elastic.go -destination=../mocks/repository/elastic.go -package=mocks

type Elastic interface {
	SearchWord(word, index string) ([]string, error)
}
