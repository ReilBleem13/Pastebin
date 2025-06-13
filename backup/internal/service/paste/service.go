package paste

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"pastebin/backup/internal/domain/events"
	"pastebin/backup/internal/domain/paste"
	"sync"
	"time"
)

type pasteService struct {
	repo    paste.Repository
	cache   paste.CacheRepository
	storage paste.StorageRepository
	bus     *events.EventBus
}

// NewPasteService создает новый экземпляр сервиса паст
func NewPasteService(
	repo paste.Repository,
	cache paste.CacheRepository,
	storage paste.StorageRepository,
	bus *events.EventBus,
) paste.Service {
	return &pasteService{
		repo:    repo,
		cache:   cache,
		storage: storage,
		bus:     bus,
	}
}

func (s *pasteService) CreatePaste(ctx context.Context, dto paste.CreatePasteDTO) (*paste.Paste, error) {
	// Создаем объект в хранилище
	owner := fmt.Sprintf("%d", dto.UserID)
	if owner != "0" {
		owner = fmt.Sprintf("%s:user:%s:", dto.Visibility, owner)
	} else {
		owner = "public:"
	}

	userMetadata := map[string]string{"has_password": "false"}
	if dto.Password != nil {
		userMetadata["has_password"] = "true"
	}

	pasta, err := s.storage.CreateOne(ctx, owner, dto.Content, userMetadata)
	if err != nil {
		return nil, fmt.Errorf("unable to save the file: %v", err)
	}

	// Генерируем хеш
	hash := sha256.Sum256([]byte(pasta.Key))
	hashStr := hex.EncodeToString(hash[:])
	pasta.Hash = hashStr

	// Сохраняем в кэш
	if err := s.cache.AddText(ctx, pasta.Hash, dto.Content); err != nil {
		return nil, err
	}

	// Сохраняем в базу данных
	if err := s.repo.Create(ctx, pasta); err != nil {
		return nil, err
	}

	// Публикуем событие создания пасты
	event := events.PasteCreatedEvent{
		PasteID:    pasta.Hash,
		UserID:     dto.UserID,
		Visibility: dto.Visibility,
		CreatedAt:  time.Now(),
	}
	if err := s.bus.Publish(ctx, event); err != nil {
		// Логируем ошибку, но не прерываем операцию
		fmt.Printf("Failed to publish paste.created event: %v\n", err)
	}

	return pasta, nil
}

func (s *pasteService) GetPaste(ctx context.Context, hash string, withMeta bool) (*paste.PasteWithData, error) {
	pasta := &paste.PasteWithData{}

	// Получаем метаданные
	if withMeta {
		meta, err := s.cache.GetMeta(ctx, hash)
		if err != nil {
			// Если нет в кэше, получаем из БД
			meta, err = s.repo.GetByHash(ctx, hash)
			if err != nil {
				return nil, err
			}
			// Сохраняем в кэш
			if err := s.cache.AddMeta(ctx, meta); err != nil {
				return nil, err
			}
		}
		pasta.Metadata = *meta
	}

	// Получаем содержимое
	content, err := s.cache.GetText(ctx, hash)
	if err != nil {
		// Если нет в кэше, получаем из хранилища
		content, err = s.storage.GetOne(ctx, pasta.Metadata.Key)
		if err != nil {
			return nil, err
		}
		// Сохраняем в кэш
		if err := s.cache.AddText(ctx, hash, content); err != nil {
			return nil, err
		}
	}
	pasta.Text = string(content)

	// Увеличиваем счетчик просмотров
	if err := s.cache.IncrementViews(ctx, hash); err != nil {
		return nil, err
	}

	return pasta, nil
}

func (s *pasteService) DeletePaste(ctx context.Context, hash string) error {
	// Получаем ключ из БД
	pasta, err := s.repo.GetByHash(ctx, hash)
	if err != nil {
		return err
	}

	// Удаляем из хранилища
	if err := s.storage.DeleteOne(ctx, pasta.Key); err != nil {
		return err
	}

	// Удаляем из БД
	if err := s.repo.Delete(ctx, hash); err != nil {
		return err
	}

	// Публикуем событие удаления пасты
	event := events.PasteDeletedEvent{
		PasteID:   hash,
		UserID:    pasta.UserID,
		DeletedAt: time.Now(),
	}
	if err := s.bus.Publish(ctx, event); err != nil {
		// Логируем ошибку, но не прерываем операцию
		fmt.Printf("Failed to publish paste.deleted event: %v\n", err)
	}

	return nil
}

func (s *pasteService) DeleteManyPastas(ctx context.Context, hashes []string) error {
	var wg sync.WaitGroup
	errChan := make(chan error, len(hashes))

	for _, hash := range hashes {
		wg.Add(1)
		go func(h string) {
			defer wg.Done()
			if err := s.DeletePaste(ctx, h); err != nil {
				errChan <- err
			}
		}(hash)
	}

	wg.Wait()
	close(errChan)

	// Проверяем ошибки
	for err := range errChan {
		if err != nil {
			return err
		}
	}

	return nil
}

func (s *pasteService) ListPastas(ctx context.Context, dto paste.ListPastasDTO) ([]paste.PastaPaginated, string, error) {
	var prefix string
	if dto.UserID != nil {
		prefix = fmt.Sprintf("user:%d", *dto.UserID)
	} else {
		prefix = "public"
	}

	pastas, nextKey, err := s.storage.Paginate(ctx, dto.MaxKeys, dto.StartAfter, prefix)
	if err != nil {
		return nil, "", err
	}

	responses := make([]paste.PastaPaginated, len(pastas))
	for i, p := range pastas {
		responses[i] = paste.PastaPaginated{
			Number: i + 1,
			Pasta:  p,
		}
	}

	return responses, nextKey, nil
}

func (s *pasteService) CheckAccess(ctx context.Context, dto paste.CheckAccessDTO) (bool, error) {
	// Проверяем права доступа
	hasAccess, err := s.repo.CheckPermission(ctx, dto.UserID, dto.Hash)
	if err != nil {
		return false, err
	}

	if !hasAccess {
		return false, nil
	}

	// Если есть пароль, проверяем его
	if dto.Password != nil {
		pasta, err := s.repo.GetByHash(ctx, dto.Hash)
		if err != nil {
			return false, err
		}

		if pasta.Password != nil && *pasta.Password != *dto.Password {
			return false, nil
		}
	}

	return true, nil
}
