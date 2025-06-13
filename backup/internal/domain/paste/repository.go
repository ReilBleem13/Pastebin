package paste

import "context"

// Repository определяет интерфейс для работы с хранилищем паст
type Repository interface {
	// Create сохраняет новую пасту
	Create(ctx context.Context, paste *Paste) error

	// GetByHash получает пасту по хешу
	GetByHash(ctx context.Context, hash string) (*Paste, error)

	// GetByUserID получает пасты пользователя
	GetByUserID(ctx context.Context, userID int) ([]*Paste, error)

	// Update обновляет пасту
	Update(ctx context.Context, paste *Paste) error

	// Delete удаляет пасту
	Delete(ctx context.Context, hash string) error

	// IncrementViews увеличивает счетчик просмотров
	IncrementViews(ctx context.Context, hash string) error

	// GetVisibility получает тип видимости пасты
	GetVisibility(ctx context.Context, hash string) (string, error)

	// CheckPermission проверяет права доступа к пасте
	CheckPermission(ctx context.Context, userID int, hash string) (bool, error)

	// GetKeys получает ключи паст пользователя
	GetKeys(ctx context.Context, userID int) ([]string, error)
}

// CacheRepository определяет интерфейс для работы с кэшем
type CacheRepository interface {
	// AddText сохраняет текст пасты в кэш
	AddText(ctx context.Context, hash string, data []byte) error

	// GetText получает текст пасты из кэша
	GetText(ctx context.Context, hash string) ([]byte, error)

	// AddMeta сохраняет метаданные пасты в кэш
	AddMeta(ctx context.Context, paste *Paste) error

	// GetMeta получает метаданные пасты из кэша
	GetMeta(ctx context.Context, hash string) (*Paste, error)

	// GetViews получает количество просмотров
	GetViews(ctx context.Context, hash string) (int, error)

	// IncrementViews увеличивает счетчик просмотров в кэше
	IncrementViews(ctx context.Context, hash string) error
}

// StorageRepository определяет интерфейс для работы с объектным хранилищем
type StorageRepository interface {
	// CreateOne создает один объект
	CreateOne(ctx context.Context, owner string, data []byte, metadata map[string]string) (*Paste, error)

	// CreateMany создает множество объектов
	CreateMany(ctx context.Context, files map[string][]byte) ([]string, error)

	// GetOne получает один объект
	GetOne(ctx context.Context, key string) ([]byte, error)

	// GetMany получает множество объектов
	GetMany(ctx context.Context, keys []string) (map[string][]byte, error)

	// DeleteOne удаляет один объект
	DeleteOne(ctx context.Context, key string) error

	// DeleteMany удаляет множество объектов
	DeleteMany(ctx context.Context, keys []string) error

	// Paginate получает список объектов с пагинацией
	Paginate(ctx context.Context, maxKeys int, startAfter string, prefix string) ([]string, string, error)
}
