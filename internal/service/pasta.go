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
	"time"
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
	indexForElastic      string = "pastas" // добавить в структуру конфиг\\

	visibilityIsPrivate string = "private"
	visibilityIsPublic  string = "public"

	textPrefix string = "text"
	metaPrefix string = "meta"
	userPrefix string = "user"

	defaultLimit int = 5
	defaultPage  int = 1
)

func NewPastaService(repo *repository.Repository, logger *logging.Logger) domainservice.Pasta {
	return &PastaService{
		s3:      repo.S3,
		cache:   repo.Cache.Pasta(),
		db:      repo.Database.Pasta(),
		elastic: repo.Elastic,
		logger:  logger,
	}
}

func (p *PastaService) GetVisibility(ctx context.Context, hash string) (string, error) {
	return p.db.GetVisibility(ctx, hash)
}

func (m *PastaService) Create(ctx context.Context, req *dto.RequestCreatePasta, userID int) (*models.Pasta, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	timeNow := time.Now()
	timeExpiration, err := validate.ValidRequestCreatePasta(req, timeNow)
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

	var pastaMetadata *models.Pasta
	err = executeWithRetry(ctx, func() error {
		var err error
		pastaMetadata, err = m.s3.Store(ctx, prefix, data, options, timeNow)
		return err
	}, 3)
	if err != nil {
		return nil, fmt.Errorf("failed to store pasta in S3: %w", err)
	}
	pastaMetadata.Hash = hashing.Hash(pastaMetadata.ObjectID)
	pastaMetadata.ExpiresAt = timeExpiration
	pastaMetadata.PasswordHash = passwordHash
	pastaMetadata.Language = req.Language
	pastaMetadata.Visibility = req.Visibility
	pastaMetadata.UserID = userID

	err = executeWithRetry(ctx, func() error {
		return m.db.Create(ctx, pastaMetadata)
	}, 3)
	if err != nil {
		if deleteErr := m.s3.Delete(ctx, pastaMetadata.ObjectID); deleteErr != nil {
			m.logger.Errorf("failed to cleanup S3 file after DB error: %v", deleteErr)
		}
		return nil, fmt.Errorf("failed to create new pasta in database : %v", err)
	}

	var wg sync.WaitGroup
	errChan := make(chan error, 2)

	wg.Add(1)
	go func() {
		defer wg.Done()

		select {
		case <-ctx.Done():
			return
		default:
			err := executeWithRetry(ctx, func() error {
				return m.cache.AddText(ctx, pastaMetadata.Hash, data)
			}, 2)

			if err != nil {
				select {
				case errChan <- fmt.Errorf("failed to cache pasta after creation: %v", err):
				case <-ctx.Done():
					return
				}
			}
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()

		select {
		case <-ctx.Done():
			return
		default:
			err := executeWithRetry(ctx, func() error {
				_, err := m.cache.Views(ctx, pastaMetadata.Hash)
				return err
			}, 2)

			if err != nil {
				m.logger.Errorf("failed to ad first view: %v", err)
			}
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()

		select {
		case <-ctx.Done():
			return
		default:
			err := executeWithRetry(ctx, func() error {
				return m.elastic.NewIndex(data, pastaMetadata.ObjectID, indexForElastic, nil)
			}, 2)

			if err != nil {
				select {
				case errChan <- fmt.Errorf("failed to index pasta after creation: %w", err):
				case <-ctx.Done():
					return
				}
			}
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

func (m *PastaService) Delete(ctx context.Context, hash string) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	key, err := m.db.DeleteMetadata(ctx, hash)
	if err != nil {
		if !errors.Is(err, customerrors.ErrPastaNotFound) {
			return fmt.Errorf("failed to delete pasta from DB: %w", err)
		}
		return err
	}

	var wg sync.WaitGroup
	errChan := make(chan error, 1)

	wg.Add(1)
	go func() {
		defer wg.Done()

		select {
		case <-ctx.Done(): //гарантирует, что функция не вызовется, если контекст отменен.
			return
		default:
			err := executeWithRetry(ctx, func() error {
				return m.s3.Delete(ctx, key)
			}, 2)

			if err != nil {
				select {
				case errChan <- fmt.Errorf("failed to delete file from S3: %w", err):
				case <-ctx.Done(): // нужен только для безопасной отпарвки ошибки в канал.
					return
				}
			}
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		select {
		case <-ctx.Done():
			return
		default:
			err := executeWithRetry(ctx, func() error {
				return m.cache.DeleteViews(ctx, hash)
			}, 2)

			if err != nil {
				m.logger.Errorf("failed to delete views from hash %s: %v", hash, err)
			}
		}
	}()
	wg.Wait()
	close(errChan)

	for err := range errChan {
		if err != nil {
			return err
		}
	}
	return nil
}

// func (m *MinioService) GetMany(ctx context.Context, objectIDs []string) ([]string, error) {
// 	return m.client.GetFiles(ctx, objectIDs)
// }

// func (m *MinioService) DeleteMany(ctx context.Context, objectIDs []string) error {
// 	return m.client.DeleteFiles(ctx, objectIDs)
// }

func (m *PastaService) GetText(ctx context.Context, keyText, objectID, hash string) (*string, error) {
	text, err := m.cache.GetText(ctx, keyText)
	if err == nil {
		return text, nil
	}

	if !errors.Is(err, customerrors.ErrKeyDoesntExist) {
		m.logger.Errorf("failed to get text from cache, falling back to S3: %v", err)
	}

	text, _, err = m.s3.Get(ctx, objectID, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get file from minio: %w", err)
	}

	if err := m.cache.AddText(ctx, hash, []byte(*text)); err != nil {
		m.logger.Errorf("error in add text to cache: %v", err)
	}
	return text, nil
}

func (m *PastaService) Get(ctx context.Context, hash string, flag bool) (*models.PastaWithData, error) {
	result := models.PastaWithData{}

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

		_, err = m.cache.Views(ctx, hash)
		if err != nil {
			m.logger.Errorf("failed to increment views for pasta %s: %v", hash, err)
		}

		result.Text = *text
		result.Metadata = nil
		return &result, nil
	}

	textCh := make(chan *string, 1)
	textErrCh := make(chan error, 1)
	metaCh := make(chan *models.Pasta, 1)
	metaErrCh := make(chan error, 1)
	var wg sync.WaitGroup

	wg.Add(1)
	go func() {
		defer wg.Done()
		text, err := m.GetText(ctx, keyText, objectID, hash)
		if err != nil {
			textErrCh <- err
			return
		}
		textCh <- text
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		metadata, err := m.cache.GetMeta(ctx, keyMeta)
		if err == nil {
			metaCh <- metadata
			return
		}

		if !errors.Is(err, customerrors.ErrKeyDoesntExist) {
			m.logger.Errorf("failed to get metadata from cache, falling back to DB: %v", err)
		}

		metadata, err = m.db.GetMetadata(ctx, objectID)
		if err != nil {
			metaErrCh <- err
			return
		}

		if err := m.cache.AddMeta(ctx, metadata); err != nil {
			m.logger.Errorf("error in add metadata to cache: %v", err)
		}
		metaCh <- metadata
	}()
	wg.Wait()
	close(textCh)
	close(textErrCh)
	close(metaCh)
	close(metaErrCh)

	var (
		text     *string
		textErr  error
		metadata *models.Pasta
		metaErr  error
	)

	if t, ok := <-textCh; ok && t != nil {
		text = t
	}
	if err, ok := <-textErrCh; ok && err != nil {
		textErr = err
	}
	if meta, ok := <-metaCh; ok && meta != nil {
		metadata = meta
	}
	if err, ok := <-metaErrCh; ok && err != nil {
		metaErr = err
	}

	if textErr != nil {
		return nil, textErr
	}
	if metaErr != nil {
		return nil, metaErr
	}

	views, err := m.cache.Views(ctx, hash)
	if err != nil {
		m.logger.Errorf("failed to increment views for pasta %s: %v", hash, err)
		views = -1
	}

	result.Metadata = metadata
	result.Metadata.Views = views
	result.Text = *text
	return &result, nil
}

func (m *PastaService) Paginate(ctx context.Context, rawLimit, rawPage string, hasMetadata bool, userID *int) (*[]dto.TextsWithMetadata, error) {
	var limit, page int

	if rawLimit == "" {
		limit = defaultLimit
	} else {
		var err error
		limit, err = strconv.Atoi(rawLimit)
		if err != nil {
			return nil, customerrors.ErrInvalidQueryParament
		}
		if limit < defaultLimit {
			limit = defaultLimit
		}
	}

	if rawPage == "" {
		page = defaultPage
	} else {
		var err error
		page, err = strconv.Atoi(rawPage)
		if err != nil {
			return nil, customerrors.ErrInvalidQueryParament
		}
		if page < defaultPage {
			page = defaultPage
		}
	}
	offset := (page - 1) * limit

	var objectIDs *[]string
	if userID != nil {
		var err error
		objectIDs, err = m.db.PaginateByUserID(ctx, limit, offset, *userID)
		if err != nil {
			return nil, fmt.Errorf("failed to paginate (byUserID) from db: %w", err)
		}
	} else {
		var err error
		objectIDs, err = m.db.Paginate(ctx, limit, offset)
		if err != nil {
			return nil, fmt.Errorf("failed to paginate from db: %w", err)
		}
	}

	if objectIDs == nil {
		return nil, customerrors.ErrPastaNotFound
	}

	var (
		metadatas *[]models.Pasta
		texts     *[]dto.Entry

		metaErr error
		textErr error
		wg      sync.WaitGroup
	)

	if hasMetadata {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if userID != nil {
				metadatas, metaErr = m.db.GetManyMetadataByUserID(ctx, objectIDs, *userID)
				if metaErr != nil {
					metaErr = fmt.Errorf("failed to get metadata (userID): %w", metaErr)
				} else if metadatas == nil {
					metaErr = fmt.Errorf("metadatas are empty")
				}
			} else {
				metadatas, metaErr = m.db.GetManyMetadataPublic(ctx, objectIDs)
				if metaErr != nil {
					metaErr = fmt.Errorf("failed to get metadata: %w", metaErr)
				} else if metadatas == nil {
					metaErr = fmt.Errorf("metadatas are empty")
				}
			}
		}()
	}

	wg.Add(1)
	go func() {
		defer wg.Done()
		texts, textErr = m.s3.GetFiles(ctx, *objectIDs, prtBool(false))
		if textErr != nil {
			textErr = fmt.Errorf("failed to get pastas from minio: %w", textErr)
		} else if texts == nil {
			textErr = fmt.Errorf("texts are empty")
		}
	}()
	wg.Wait()

	if hasMetadata {
		if metaErr != nil {
			return nil, metaErr
		}
		if textErr != nil {
			return nil, textErr
		}

		for i := range *metadatas {
			hash := (*metadatas)[i].Hash
			views, err := m.cache.GetViews(ctx, hash)
			if err != nil {
				m.logger.Errorf("failed to get views for hash %s: %v", hash, err)
				(*metadatas)[i].Views = -1
				continue
			}
			viewsInt, err := strconv.Atoi(views)
			if err != nil {
				m.logger.Errorf("failed to convert string views to int for hash %s: %v", hash, err)
				(*metadatas)[i].Views = -1
				continue
			}
			(*metadatas)[i].Views = viewsInt
		}

	} else {
		if textErr != nil {
			return nil, textErr
		}
	}
	result := utils.MergeEntriesWithPasta(texts, metadatas)
	return result, nil
}

func prtBool(b bool) *bool {
	return &b
}

func executeWithRetry(ctx context.Context, operation func() error, maxRetries int) error {
	var lastErr error

	for attempt := 0; attempt < maxRetries; attempt++ {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			if err := operation(); err == nil {
				return nil
			} else {
				lastErr = err

				if attempt < maxRetries-1 {
					delay := time.Duration(attempt+1) * time.Second

					select {
					case <-time.After(delay):
					case <-ctx.Done():
						return ctx.Err()
					}
				}
			}
		}
	}
	return fmt.Errorf("failed after %d attemps, last error: %w", maxRetries, lastErr)
}
