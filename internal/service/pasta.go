package service

import (
	"context"
	"errors"
	"fmt"
	domainrepo "pastebin/internal/domain/repository"
	domainservice "pastebin/internal/domain/service"
	customerrors "pastebin/internal/errors"
	"pastebin/internal/models"
	"pastebin/internal/repository"
	"pastebin/internal/utils"
	"pastebin/pkg/dto"
	hashing "pastebin/pkg/hash"
	"pastebin/pkg/logging"
	"pastebin/pkg/validate"
	"strconv"
	"sync"
)

type PastaService struct {
	s3      domainrepo.S3
	cache   domainrepo.PastaCache
	db      domainrepo.PastaDatabase
	elastic domainrepo.Elastic
	logger  *logging.Logger
}

const (
	defaultNewFilePrefix string = "public:"
	indexForElastic      string = "pastas" // добавить в структуру конфиг
	visibilityIsPrivate  string = "private"

	textPrefix string = "text"
	metaPrefix string = "meta"
	userPrefix string = "user"

	defaultMaxObject int = 5
)

func NewPastaService(repo *repository.Repository, logger *logging.Logger) domainservice.Pasta {
	return &PastaService{
		s3:      repo.S3,
		cache:   repo.Cache.Pasta(),
		db:      repo.Database.Pasta(),
		elastic: repo.Elastic,
	}
}

func (p *PastaService) GetVisibility(ctx context.Context, hash string) (string, error) {
	return p.db.GetVisibility(ctx, hash)
}

func (m *PastaService) Create(ctx context.Context, req *dto.RequestCreatePasta, userID int) (*models.Pasta, error) {
	timeExpiration, err := validate.ValidRequestCreatePasta(req)
	if err != nil {
		return nil, err
	}

	var prefix string
	if userID != 0 {
		prefix = fmt.Sprintf("%s:user:%d:", req.Visibility, userID)
	} else {
		prefix = defaultNewFilePrefix
	}

	options := map[string]string{
		"has_password": "false",
	}

	var passwordHash string
	if req.Password != "" {
		options["has_password"] = "true"

		var err error
		passwordHash, err = utils.HashPassword(req.Password)
		if err != nil {
			return nil, fmt.Errorf("failed to hash password: %w", err)
		}
	}

	data := []byte(req.Message)
	pastaMetadata, err := m.s3.Store(ctx, prefix, data, options)
	if err != nil {
		return nil, fmt.Errorf("failed to store pasta in S3: %w", err)
	}
	pastaMetadata.Hash = hashing.Hash(pastaMetadata.ObjectID)
	pastaMetadata.ExpiresAt = timeExpiration
	pastaMetadata.PasswordHash = passwordHash
	pastaMetadata.Language = req.Language
	pastaMetadata.Visibility = req.Visibility
	pastaMetadata.UserID = userID

	// добавление метаданных в бд
	if err := m.db.Create(ctx, pastaMetadata); err != nil {
		// добавить удаление с minio + логировать если ошибка
		return nil, fmt.Errorf("failed to create new pasta in database: %w", err)
	}

	var wg sync.WaitGroup
	errChan := make(chan error, 2)

	// кеширование текста
	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := m.cache.AddText(ctx, pastaMetadata.Hash, data); err != nil {
			errChan <- fmt.Errorf("failed to cache pasta after creation: %w", err)
		}
	}()

	// индексирование текста
	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := m.elastic.NewIndex(data, pastaMetadata.ObjectID, indexForElastic, nil); err != nil {
			errChan <- fmt.Errorf("failed to index pasta after creation: %w", err)
		}
	}()
	wg.Wait()
	close(errChan)

	for err := range errChan {
		if err != nil {
			m.logger.Errorf("error in background pasta creation task: %v", err)
		}
	}
	return pastaMetadata, nil
}

func (m *PastaService) Permission(ctx context.Context, hash, password, visibility string, userID int) error {
	// проверяем существования пасты
	pastaExists, err := m.db.IsPastaExists(ctx, hash)
	if err != nil {
		return fmt.Errorf("failed to check is pasta exists: %w", err)
	}
	if !pastaExists {
		return customerrors.ErrPastaNotFound
	}

	// проверка, если паста приватная
	if visibility == visibilityIsPrivate {
		exists, err := m.db.IsAccessPrivate(ctx, userID, hash)
		if err != nil {
			return fmt.Errorf("failed to check is accecc private: %w", err)
		}
		if !exists {
			return customerrors.ErrNoAccess
		}
		return nil
	}

	// проверка, если паста публичная
	passwordHash, err := m.db.GetPassword(ctx, hash)
	if err != nil && !errors.Is(err, customerrors.ErrPasswordIsEmpty) {
		return fmt.Errorf("failed to get password: %w", err)
	}

	if passwordHash != "" {
		if !utils.CheckPasswordHash(password, passwordHash) {
			return customerrors.ErrWrongPassword
		}
		return nil
	}
	return nil
}

func (m *PastaService) GetText(ctx context.Context, keyText, objectID, hash string) (*string, error) {
	text, err := m.cache.GetText(ctx, keyText)
	if err == nil {
		return text, nil
	}
	if !errors.Is(err, customerrors.ErrKeyDoesntExist) {
		m.logger.Errorf("failed to get text from cache, falling back to S3: %v", err)
	}

	text, err = m.s3.Get(ctx, objectID)
	if err != nil {
		return nil, fmt.Errorf("failed to get file from minio: %w", err)
	}
	m.logger.Info("Текст из S3") // убрать

	if err := m.cache.AddText(ctx, hash, []byte(*text)); err != nil {
		m.logger.Errorf("error in add text to cache: %v", err)
	}
	m.logger.Info("Текст добавлен в Cache") // убрать
	return text, nil
}

func (m *PastaService) Get(ctx context.Context, hash string, flag bool) (*models.PastaWithData, error) {
	result := &models.PastaWithData{}

	objectID, err := m.db.GetKey(ctx, hash)
	if err != nil {
		return nil, fmt.Errorf("failed to get objectID: %w", err)
	}

	keyMeta := fmt.Sprintf("%s:%s", metaPrefix, hash)
	keyText := fmt.Sprintf("%s:%s", textPrefix, hash)

	if !flag {
		text, err := m.GetText(ctx, keyText, objectID, hash)
		if err != nil {
			return nil, err
		}
		// только инкримент просмотров
		_, err = m.cache.Views(ctx, hash)
		if err != nil {
			m.logger.Errorf("failed to increment views for pasta %s: %v", hash, err)
		}

		result.Text = *text
		result.Metadata = nil
		return result, nil
	}

	textCh := make(chan *string, 1)
	textErrCh := make(chan error, 1)
	metaCh := make(chan *models.Pasta, 1)
	metaErrCh := make(chan error, 1)

	go func() {
		text, err := m.GetText(ctx, keyText, objectID, hash)
		if err != nil {
			textErrCh <- err
			return
		}
		textCh <- text
	}()
	go func() {
		metadata, err := m.cache.GetMeta(ctx, keyMeta)
		if err == nil {
			m.logger.Info("Метаданые получены из Cache") //убрать
			metaCh <- metadata
			return
		}
		if !errors.Is(err, customerrors.ErrKeyDoesntExist) {
			m.logger.Errorf("failed to get metadata from cache, falling back to DB: %v", err)
		}

		metadata, err = m.db.GetMetadata(ctx, objectID)
		if err != nil {
			m.logger.Info("Метаданые получены из DB") //убрать
			metaErrCh <- err
			return
		}
		if err := m.cache.AddMeta(ctx, metadata); err != nil {
			m.logger.Errorf("error in add metadata to cache: %v", err)
		}
		metaCh <- metadata
	}()
	var text *string
	var metadata *models.Pasta
	for i := 0; i < 2; i++ {
		select {
		case t := <-textCh:
			text = t
		case err := <-textErrCh:
			return nil, err
		case m := <-metaCh:
			metadata = m
		case err := <-metaErrCh:
			return nil, err
		}
	}

	views, err := m.cache.Views(ctx, hash)
	if err != nil {
		m.logger.Errorf("failed to increment views for pasta %s: %v", hash, err)
		views = -1
	}
	result.Metadata = metadata
	result.Metadata.Views = views
	result.Text = *text

	return result, nil
}

func (m *PastaService) Delete(ctx context.Context, hash string) error {
	key, err := m.db.DeleteMetadata(ctx, hash)
	if err != nil {
		if !errors.Is(err, customerrors.ErrPastaNotFound) {
			return fmt.Errorf("failed to delete pasta from DB: %w", err)
		}
		return err
	}
	if err := m.s3.Delete(ctx, key); err != nil {
		return fmt.Errorf("failed to delete file from S3: %w", err)
	}
	return nil
}

// func (m *MinioService) GetMany(ctx context.Context, objectIDs []string) ([]string, error) {
// 	return m.client.GetFiles(ctx, objectIDs)
// }

// func (m *MinioService) DeleteMany(ctx context.Context, objectIDs []string) error {
// 	return m.client.DeleteFiles(ctx, objectIDs)
// }

func (m *PastaService) Paginate(ctx context.Context, rawLimit, startAfter string, userID *int) (*[]models.PastaPaginated, string, error) {
	var limit int

	if rawLimit == "" {
		limit = defaultMaxObject
	} else {
		var err error
		limit, err = strconv.Atoi(rawLimit)
		if err != nil {
			return nil, "", customerrors.ErrInvalidQueryParament
		}
		if limit < 5 {
			limit = defaultMaxObject
		}
	}

	exists, err := m.db.IsPastaExistsByObjectID(ctx, startAfter)
	if err != nil {
		return nil, "", fmt.Errorf("failed to check is pasta exists by objectID: %w", err)
	}

	if !exists {
		return nil, "", customerrors.ErrPastaNotFound
	}

	prefix := visibilityIsPrivate

	if userID != nil {
		prefix := fmt.Sprintf("%s:%d", userPrefix, *userID)
		pastas, nextKey, err := m.s3.PaginateFilesByUserID(ctx, limit, startAfter, prefix)
		if err != nil {
			return nil, "", fmt.Errorf("failed to paginate files by userID: %w", err)
		}
		responses := formResponse(pastas)
		return responses, nextKey, nil
	}

	pastas, nextKey, err := m.s3.PaginateFiles(ctx, limit, startAfter, prefix)
	if err != nil {
		return nil, "", err
	}
	responses := formResponse(pastas)
	return responses, nextKey, nil
}

func formResponse(pastas *[]string) *[]models.PastaPaginated {
	responses := []models.PastaPaginated{}

	for i, pasta := range *pastas {
		response := models.PastaPaginated{
			Number: i + 1,
			Pasta:  pasta,
		}
		responses = append(responses, response)
	}
	return &responses
}

// func prtSrt(s string) *string {
// 	return &s
// }
