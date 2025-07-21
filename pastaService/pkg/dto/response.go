package dto

import (
	"pastebin/internal/models"
	"time"
)

type SuccessCreatePastaResponse struct {
	Status   int              `json:"status"`
	Message  string           `json:"message"`
	Link     string           `json:"link"`
	Metadata PastaMetadataDTO `json:"metadata,omitempty"`
}

type SuccessUpdatedPastaResponse struct {
	Status   int          `json:"status"`
	Message  string       `json:"message"`
	Metadata models.Pasta `json:"metadata"`
}

type PastaMetadataDTO struct {
	Key        string    `json:"key"`
	Size       int       `json:"size"`
	Language   string    `json:"language"`
	Visibility string    `json:"visibility"`
	CreatedAt  time.Time `json:"created_at"`
	ExpiresAt  time.Time `json:"expires_at"`
}

type GetPastaResponse struct {
	Status   int           `json:"status"`
	Message  string        `json:"message"`
	Text     string        `json:"text"`
	Metadata *models.Pasta `json:"metadata,omitempty"`
}

type PaginatedPastaDTO struct {
	Status int                 `json:"status"`
	Pastas []TextsWithMetadata `json:"result"`
	Page   int                 `json:"page"`
	Limit  int                 `json:"limit"`
	Total  int                 `json:"total"`
}

type Entry struct {
	Text     string    `json:"text"`
	ObjectID string    `json:"-"`
	Time     time.Time `json:"-"`
}

type TextsWithMetadata struct {
	Text     string        `json:"text"`
	Metadata *models.Pasta `json:"metadata,omitempty"`
}

type SearchedPastas struct {
	Status int      `json:"status"`
	Pastas []string `json:"pastas"`
}
