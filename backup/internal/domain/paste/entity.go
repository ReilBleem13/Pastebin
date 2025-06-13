package paste

import "time"

// Paste представляет собой основную сущность пасты
type Paste struct {
	ID          string
	UserID      int
	Content     []byte
	Visibility  string
	Password    *string
	CreatedAt   time.Time
	UpdatedAt   time.Time
	Views       int
	Hash        string
	Key         string
	HasPassword bool
}

// PasteWithData расширяет Paste дополнительными данными
type PasteWithData struct {
	Metadata Paste
	Text     string
}

// PastaPaginated представляет пасту для пагинации
type PastaPaginated struct {
	Number int
	Pasta  string
}

// Visibility константы для типов видимости
const (
	VisibilityPublic   = "public"
	VisibilityPrivate  = "private"
	VisibilityUnlisted = "unlisted"
)
