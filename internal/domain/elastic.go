package domain

type Elastic interface {
	NewIndex(text *[]byte, key, index string, tags []string) error
	SearchWord(word, index string) ([]string, error)
}
