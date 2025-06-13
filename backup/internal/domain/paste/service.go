package paste

import "context"

// Service определяет интерфейс сервиса для работы с пастами
type Service interface {
	// CreatePaste создает новую пасту
	CreatePaste(ctx context.Context, dto CreatePasteDTO) (*Paste, error)

	// GetPaste получает пасту по хешу
	GetPaste(ctx context.Context, hash string, withMeta bool) (*PasteWithData, error)

	// DeletePaste удаляет пасту
	DeletePaste(ctx context.Context, hash string) error

	// DeleteManyPastas удаляет множество паст
	DeleteManyPastas(ctx context.Context, hashes []string) error

	// ListPastas получает список паст с пагинацией
	ListPastas(ctx context.Context, dto ListPastasDTO) ([]PastaPaginated, string, error)

	// CheckAccess проверяет доступ к пасте
	CheckAccess(ctx context.Context, dto CheckAccessDTO) (bool, error)
}

// CreatePasteDTO представляет данные для создания пасты
type CreatePasteDTO struct {
	UserID     int
	Content    []byte
	Visibility string
	Password   *string
}

// ListPastasDTO представляет параметры для получения списка паст
type ListPastasDTO struct {
	MaxKeys    int
	StartAfter string
	UserID     *int
}

// CheckAccessDTO представляет параметры для проверки доступа
type CheckAccessDTO struct {
	UserID   int
	Hash     string
	Password *string
}
