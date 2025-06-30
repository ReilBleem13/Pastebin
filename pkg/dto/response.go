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
	Status       int                     `json:"status"`
	NextObjectID string                  `json:"object_id"`
	Pastas       []models.PastaPaginated `json:"pasta"`
}
