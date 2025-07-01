package utils

import (
	"pastebin/internal/models"
	"pastebin/pkg/dto"
)

func MergeEntriesWithPasta(entries *[]dto.Entry, pastas *[]models.Pasta) *[]dto.TextsWithMetadata {
	var pastaMap map[string]models.Pasta

	if pastas != nil {
		pastaMap = make(map[string]models.Pasta, len(*pastas))
		for _, pasta := range *pastas {
			pastaMap[pasta.ObjectID] = pasta
		}
	} else {
		pastaMap = make(map[string]models.Pasta)
	}

	var result []dto.TextsWithMetadata
	if entries != nil {
		for _, entry := range *entries {
			var metadata *models.Pasta
			if pasta, ok := pastaMap[entry.ObjectID]; ok {
				metadata = &pasta
			} else {
				metadata = nil
			}
			result = append(result, dto.TextsWithMetadata{
				Text:     entry.Text,
				Metadata: metadata,
			})
		}
	}
	return &result
}
