package service

import (
	"bytes"
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
	"pastebin/pkg/retry"
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
	indexForElastic      string = "pastas"

	link string = "localhost:10002/receive/"

	privateVisibility string = "private"

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

func (p *PastaService) GetUserID(ctx context.Context, hash string) (int, error) {
	return p.db.GetUserID(ctx, hash)
}

func (m *PastaService) Create(ctx context.Context, req *dto.RequestCreatePasta, userID int) (*models.Pasta, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	expiration, err := validate.ValidRequestCreatePasta(req)
	if err != nil {
		return nil, err
	}

	timeNow := time.Now()
	expiresAt := timeNow.Add(expiration)

	var prefix string
	if userID != 0 {
		prefix = fmt.Sprintf("%s:user:%d:", req.Visibility, userID)
	} else {
		prefix = defaultNewFilePrefix
	}

	options := map[string]string{"has_password": "false"}
	var passwordHash string
	if req.Password != "" {
		passwordHash, err = utils.HashPassword(req.Password)
		if err != nil {
			return nil, fmt.Errorf("failed to hash password: %w", err)
		}
		options["has_password"] = "true"
	}

	data := []byte(req.Message)
	var pastaMetadata *models.Pasta
	err = retry.Retry(ctx, func() error {
		var s3Err error
		pastaMetadata, s3Err = m.s3.Store(ctx, prefix, data, options, timeNow)
		return s3Err
	}, retry.IsRetryableErrorMinio, retry.NewConfigWithComponent("s3"), m.logger)
	if err != nil {
		return nil, fmt.Errorf("failed to store pasta in S3: %w", err)
	}

	pastaMetadata.Hash = hashing.Hash(pastaMetadata.ObjectID)
	pastaMetadata.ExpiresAt = expiresAt
	pastaMetadata.ExpireAfterRead = req.ExpireAfterRead
	pastaMetadata.PasswordHash = passwordHash
	pastaMetadata.Language = req.Language
	pastaMetadata.Visibility = req.Visibility
	pastaMetadata.UserID = userID

	err = retry.Retry(ctx, func() error {
		return m.db.Create(ctx, pastaMetadata)
	}, retry.IsRetryableErrorDatabase, retry.NewConfigWithComponent("db"), m.logger)
	if err != nil {
		if deleteErr := m.s3.Delete(ctx, pastaMetadata.ObjectID); deleteErr != nil {
			m.logger.Errorf("Failed to cleanup S3 file after DB error: %v, hash: %s", deleteErr, pastaMetadata.Hash)
		}
		return nil, fmt.Errorf("failed to create new pasta in database : %w", err)
	}

	go func() {
		if err := retry.Retry(ctx, func() error {
			return m.cache.AddText(ctx, pastaMetadata.Hash, data)
		}, retry.IsRetryableErrorRedis, retry.NewConfigWithComponent("cache"), m.logger); err != nil {
			m.logger.Errorf("Failed to cache pasta: %v, hash=%s", err, pastaMetadata.Hash)
		}
	}()

	go func() {
		if err := retry.Retry(ctx, func() error {
			err := m.cache.CreateViews(ctx, pastaMetadata.Hash, &expiration)
			return err
		}, retry.IsRetryableErrorRedis, retry.NewConfigWithComponent("cache"), m.logger); err != nil {
			m.logger.Errorf("Failed to add views: %v, hash=%s", err, pastaMetadata.Hash)
		}
	}()

	if err := retry.Retry(ctx, func() error {
		err := m.elastic.Indexing(data, pastaMetadata.ObjectID)
		return err
	}, retry.IsRetryableErrorElastic, retry.NewConfigWithComponent("elastic"), m.logger); err != nil {
		return nil, fmt.Errorf("failed to index pasta: %w", err)
	}
	return pastaMetadata, nil
}

func (m *PastaService) Permission(ctx context.Context, hash, password, visibility string, userID int) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	var exists bool
	err := retry.Retry(ctx, func() error {
		var err error
		exists, err = m.db.IsPastaExists(ctx, hash)
		return err
	}, retry.IsRetryableErrorDatabase, retry.NewConfigWithComponent("db"), m.logger)
	if err != nil {
		return fmt.Errorf("failed to check is pasta exists: %w", err)
	}
	if !exists {
		return customerrors.ErrPastaNotFound
	}

	if visibility == privateVisibility {
		var hasAccess bool
		err = retry.Retry(ctx, func() error {
			var err error
			hasAccess, err = m.db.IsAccessPrivate(ctx, userID, hash)
			return err
		}, retry.IsRetryableErrorDatabase, retry.NewConfigWithComponent("db"), m.logger)
		if err != nil {
			return fmt.Errorf("failed to check, accecc is private: %w", err)
		}
		if !hasAccess {
			return customerrors.ErrNoAccess
		}
		return nil
	}

	var hashPassword string
	err = retry.Retry(ctx, func() error {
		var err error
		hashPassword, err = m.db.GetPassword(ctx, hash)
		return err
	}, retry.IsRetryableErrorDatabase, retry.NewConfigWithComponent("db"), m.logger)
	if err != nil && !errors.Is(err, customerrors.ErrPasswordIsEmpty) {
		return fmt.Errorf("failed to get password: %w", err)
	}

	if hashPassword != "" && !utils.CheckPasswordHash(password, hashPassword) {
		return customerrors.ErrWrongPassword
	}
	return nil
}

func (m *PastaService) Delete(ctx context.Context, hash string) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	var key string
	err := retry.Retry(ctx, func() error {
		var err error
		key, err = m.db.DeleteMetadata(ctx, hash)
		return err
	}, retry.IsRetryableErrorDatabase, retry.NewConfigWithComponent("db"), m.logger)
	if err != nil {
		if !errors.Is(err, customerrors.ErrPastaNotFound) {
			return fmt.Errorf("failed to delete pasta from DB: %w", err)
		}
		return err
	}

	var wg sync.WaitGroup
	errChan := make(chan error, 2)

	wg.Add(1)
	go func() {
		defer wg.Done()

		if err := retry.Retry(ctx, func() error {
			return m.s3.Delete(ctx, key)
		}, retry.IsRetryableErrorMinio, retry.NewConfigWithComponent("s3"), m.logger); err != nil {
			errChan <- fmt.Errorf("failed to delete pasta from S3: %w", err)
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()

		if err := retry.Retry(ctx, func() error {
			err := m.elastic.DeleteDocument(ctx, key)
			return err
		}, retry.IsRetryableErrorElastic, retry.NewConfigWithComponent("elastic"), m.logger); err != nil {
			errChan <- fmt.Errorf("failed to delete pasta from elastic: %w", err)
		}
	}()
	wg.Wait()
	close(errChan)

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()

		if err := retry.Retry(ctx, func() error {
			return m.cache.DeleteViews(ctx, hash)
		}, retry.IsRetryableErrorRedis, retry.NewConfigWithComponent("cache"), m.logger); err != nil {
			m.logger.Errorf("failed to delete views from hash %s: %v", hash, err)
		}
	}()

	var errs []error
	for err := range errChan {
		if err != nil {
			errs = append(errs, err)
		}
	}

	if len(errs) > 0 {
		m.logger.Error("some delete steps failed", "errors:", errs)
		return errs[0]
	}
	return nil
}

func (m *PastaService) GetText(ctx context.Context, keyText, objectID, hash string) (string, error) {
	var text string

	err := retry.Retry(ctx, func() error {
		var err error
		text, err = m.cache.GetText(ctx, keyText)
		return err
	}, retry.IsRetryableErrorRedis, retry.NewConfigWithComponent("cache"), m.logger)
	if err == nil {
		return text, nil
	}

	if !errors.Is(err, customerrors.ErrKeyDoesntExist) {
		m.logger.Warnf("failed to get text from cache (fallback to s3): %v, hash: %s", err, hash)
	}

	err = retry.Retry(ctx, func() error {
		text, err = m.s3.Get(ctx, objectID)
		return err
	}, retry.IsRetryableErrorMinio, retry.NewConfigWithComponent("s3"), m.logger)
	if err != nil {
		return "", fmt.Errorf("failed to get file from minio: %w", err)
	}

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()

		err := retry.Retry(ctx, func() error {
			return m.cache.AddText(ctx, hash, []byte(text))
		}, retry.IsRetryableErrorRedis, retry.NewConfigWithComponent("cache"), m.logger)
		if err != nil {
			m.logger.Errorf("failed to add text to cache: %v, hash: %s", err, hash)
		}
	}()
	return text, nil
}

func (m *PastaService) GetMetadata(ctx context.Context, keyMeta string, hash string) (*models.Pasta, error) {
	var metadata *models.Pasta
	err := retry.Retry(ctx, func() error {
		var err error
		metadata, err = m.cache.GetMeta(ctx, keyMeta)
		return err
	}, retry.IsRetryableErrorRedis, retry.NewConfigWithComponent("cache"), m.logger)
	if err == nil {
		return metadata, err
	}

	if !errors.Is(err, customerrors.ErrKeyDoesntExist) {
		m.logger.Warnf("failed to get metadata from cache (falling back to DB): %v, hash: %s", err, hash)
	}

	err = retry.Retry(ctx, func() error {
		metadata, err = m.db.GetMetadata(ctx, hash)
		return err
	}, retry.IsRetryableErrorDatabase, retry.NewConfigWithComponent("db"), m.logger)
	if err != nil {
		return nil, err
	}

	go func() {
		err = retry.Retry(ctx, func() error {
			return m.cache.AddMeta(ctx, metadata)
		}, retry.IsRetryableErrorRedis, retry.NewConfigWithComponent("cache"), m.logger)
		if err != nil {
			m.logger.Errorf("failed to add metadata to cache: %v, hash: %s", err, hash)
		}
	}()
	return metadata, nil
}

func (m *PastaService) Get(ctx context.Context, hash string, withMetadata bool) (*models.PastaWithData, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	var result models.PastaWithData
	var metadata *models.Pasta
	var views int

	var objectID string
	err := retry.Retry(ctx, func() error {
		var err error
		objectID, err = m.db.GetKey(ctx, hash)
		return err
	}, retry.IsRetryableErrorDatabase, retry.NewConfigWithComponent("db"), m.logger)
	if err != nil {
		return nil, fmt.Errorf("failed to get objectID: %w", err)
	}

	keyMeta := fmt.Sprintf("%s:%s", metaPrefix, hash)
	keyText := fmt.Sprintf("%s:%s", textPrefix, hash)

	text, err := m.GetText(ctx, keyText, objectID, hash)
	if err != nil {
		return nil, err
	}

	err = retry.Retry(ctx, func() error {
		views, err = m.cache.IncrViews(ctx, hash)
		return err
	}, retry.IsRetryableErrorRedis, retry.NewConfigWithComponent("cache"), m.logger)
	if err != nil {
		m.logger.Errorf("failed to increment views for pasta %s: %v", hash, err)
		views = -1
	}

	result.Text = text
	if withMetadata {
		var err error
		metadata, err = m.GetMetadata(ctx, keyMeta, hash)
		if err != nil {
			m.logger.Errorf("failed to get metadata: %v", err)
			return nil, err
		}
		result.Metadata = metadata
		result.Metadata.Views = views
	}

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		var isExpireAfterRead bool
		var err error

		if metadata != nil {
			isExpireAfterRead = metadata.ExpireAfterRead
		} else {
			err = retry.Retry(ctx, func() error {
				var err error
				isExpireAfterRead, err = m.db.GetExpireAfterReadField(ctx, hash)
				return err
			}, retry.IsRetryableErrorDatabase, retry.NewConfigWithComponent("db"), m.logger)
			if err != nil {
				m.logger.Errorf("failed to expire-after-read flag for hash=%s: %v", hash, err)
				return
			}
		}

		if isExpireAfterRead {
			m.logger.Info("(expire after read) deleting...")
			if err = m.Delete(ctx, hash); err != nil {
				m.logger.Errorf("failed to delete pasta after read fro hash=%s: %v", hash, err)
			}
		}
	}()
	return &result, nil
}

func (m *PastaService) Search(ctx context.Context, word string) ([]string, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	var objectIDs []string
	err := retry.Retry(ctx, func() error {
		var err error
		objectIDs, err = m.elastic.SearchWord(word)
		return err
	}, retry.IsRetryableErrorElastic, retry.NewConfigWithComponent("elastic"), m.logger)
	if err != nil {
		return nil, fmt.Errorf("failed to search word: %w", err)
	}

	if len(objectIDs) == 0 {
		return []string{}, customerrors.ErrEmptySearchResult
	}

	var hashes []string
	err = retry.Retry(ctx, func() error {
		hashes, err = m.db.GetPublicHashs(ctx, objectIDs)
		return err
	}, retry.IsRetryableErrorDatabase, retry.NewConfigWithComponent("db"), m.logger)
	if err != nil {
		return nil, fmt.Errorf("failed to get hashs from db: %w", err)
	}

	for i := range hashes {
		hashes[i] = link + hashes[i]
	}
	return hashes, nil
}

func (m *PastaService) Update(ctx context.Context, newText []byte, hash string) (*models.Pasta, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	var objectID string
	err := retry.Retry(ctx, func() error {
		var err error
		objectID, err = m.db.GetKey(ctx, hash)
		return err
	}, retry.IsRetryableErrorDatabase, retry.NewConfigWithComponent("db"), m.logger)
	if err != nil {
		return nil, fmt.Errorf("failed to get objectID from db: %w, hash=%s", err, hash)
	}

	newTextBytes := bytes.NewReader(newText)

	err = retry.Retry(ctx, func() error {
		return m.s3.Update(ctx, newTextBytes, objectID)
	}, retry.IsRetryableErrorMinio, retry.NewConfigWithComponent("s3"), m.logger)
	if err != nil {
		return nil, fmt.Errorf("failed to update s3 value: %w, hash=%s", err, hash)
	}

	var metadata *models.Pasta
	err = retry.Retry(ctx, func() error {
		metadata, err = m.db.UpdateSizeAndReturnAll(ctx, hash, int(newTextBytes.Size()))
		return err
	}, retry.IsRetryableErrorDatabase, retry.NewConfigWithComponent("db"), m.logger)
	if err != nil {
		return nil, fmt.Errorf("failed to update db: %w, hash=%s", err, hash)
	}

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		err := retry.Retry(ctx, func() error {
			return m.cache.AddText(ctx, hash, newText)
		}, retry.IsRetryableErrorRedis, retry.NewConfigWithComponent("cache"), m.logger)
		if err != nil {
			m.logger.Errorf("failed to set new text to cache: %v, hash=%s", err, hash)
		}
	}()
	return metadata, nil
}

func (m *PastaService) Favorite(ctx context.Context, hash, visibility string, userID int) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	if visibility == privateVisibility {
		return customerrors.ErrNotAllowed
	}

	var exists bool
	err := retry.Retry(ctx, func() error {
		var err error
		exists, err = m.db.IsPastaExists(ctx, hash)
		return err
	}, retry.IsRetryableErrorDatabase, retry.NewConfigWithComponent("db"), m.logger)
	if err != nil {
		return fmt.Errorf("failed check is pasta exists: %w, hash=%s", err, hash)
	}

	if !exists {
		return customerrors.ErrPastaNotFound
	}
	return m.db.Favorite(ctx, hash, userID)
}

func (m *PastaService) GetFavorite(ctx context.Context, userID, favoriteID int, withMetadata bool) (*models.PastaWithData, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	var hash string
	err := retry.Retry(ctx, func() error {
		var err error
		hash, err = m.db.GetFavoriteAndCheckUser(ctx, userID, favoriteID)
		return err
	}, retry.IsRetryableErrorDatabase, retry.NewConfigWithComponent("db"), m.logger)
	if err != nil {
		return nil, fmt.Errorf("failed to get favorite and check user: %w", err)
	}

	pasta, err := m.Get(ctx, hash, withMetadata)
	if err != nil {
		return nil, fmt.Errorf("failed to get pasta: %w, hash=%s", err, hash)
	}
	return pasta, nil
}

func (m *PastaService) DeleteFavorite(ctx context.Context, userID, favoriteID int) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	err := retry.Retry(ctx, func() error {
		return m.db.DeleteFavorite(ctx, userID, favoriteID)
	}, retry.IsRetryableErrorDatabase, retry.NewConfigWithComponent("db"), m.logger)
	if err != nil {
		return fmt.Errorf("failed to delete favorite: %w", err)
	}
	return nil
}

func (m *PastaService) Paginate(
	ctx context.Context, rawLimit, rawPage string, userID int,
	hasMetadata bool, paginateFn models.PaginateFunc,
) (*dto.PaginatedPastaDTO, error) {

	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	var (
		limit = defaultLimit
		page  = defaultPage
		err   error
	)

	if rawLimit != "" {
		limit, err := strconv.Atoi(rawLimit)
		if err != nil {
			return nil, customerrors.ErrInvalidQueryParament
		}
		if limit < defaultLimit {
			limit = defaultLimit
		}
	}

	if rawPage != "" {
		page, err = strconv.Atoi(rawPage)
		if err != nil {
			return nil, customerrors.ErrInvalidQueryParament
		}
		if page < defaultPage {
			page = defaultPage
		}
	}
	offset := (page - 1) * limit

	var objectIDs []string
	if err := retry.Retry(ctx, func() error {
		objectIDs, err = paginateFn(ctx, limit, offset, userID)
		return err
	}, retry.IsRetryableErrorDatabase, retry.NewConfigWithComponent("db"), m.logger); err != nil {
		return nil, fmt.Errorf("failed to get object ids from db: %w", err)
	}

	if objectIDs == nil {
		return nil, customerrors.ErrPastaNotFound
	}

	texts := make([]string, 0, len(objectIDs))
	metadatas := make([]models.Pasta, len(objectIDs))

	var metaErr error

	wg := &sync.WaitGroup{}
	if hasMetadata {
		wg.Add(1)
		go func() {
			defer wg.Done()

			var err error
			metadatas, err = func() ([]models.Pasta, error) {
				var rawMetadatas []models.Pasta
				err := retry.Retry(ctx, func() error {
					var innerErr error
					rawMetadatas, innerErr = m.db.GetManyMetadata(ctx, objectIDs)
					return innerErr
				}, retry.IsRetryableErrorDatabase, retry.NewConfigWithComponent("db"), m.logger)
				return rawMetadatas, err
			}()
			if err != nil {
				metaErr = err
				return
			}

			for i := range metadatas {
				viewsStr := ""
				err := retry.Retry(ctx, func() error {
					var innerErr error
					viewsStr, innerErr = m.cache.GetViews(ctx, metadatas[i].Hash)
					return innerErr
				}, retry.IsRetryableErrorRedis, retry.NewConfigWithComponent("cache"), m.logger)

				if err != nil {
					m.logger.Errorf("failed to get views for hash=%s: %v", metadatas[i].Hash, err)
					metadatas[i].Views = -1
					continue
				}

				viewsInt, err := strconv.Atoi(viewsStr)
				if err != nil {
					m.logger.Errorf("failed to convert string views to int for hash=%s: %v", metadatas[i].Hash, err)
					metadatas[i].Views = -1
					continue
				}
				metadatas[i].Views = viewsInt
			}

		}()
	}

	if err := retry.Retry(ctx, func() error {
		var err error
		texts, err = m.s3.GetFiles(ctx, objectIDs)
		return err
	}, retry.IsRetryableErrorMinio, retry.NewConfigWithComponent("s3"), m.logger); err != nil {
		return nil, fmt.Errorf("failed to get files from s3: %w", err)
	}

	wg.Wait()

	if metaErr != nil {
		return nil, fmt.Errorf("failed to get metadata: %w", metaErr)
	}

	paginatedResult := mergeTextWithMetadata(texts, metadatas, hasMetadata)
	return &dto.PaginatedPastaDTO{
		Status: 200,
		Pastas: paginatedResult,
		Page:   page,
		Limit:  limit,
		Total:  len(paginatedResult),
	}, nil
}

func (m *PastaService) PaginateFavorite(ctx context.Context, rawLimit, rawPage string, userID int, hasMetadata bool) (*dto.PaginatedPastaDTO, error) {
	return m.Paginate(ctx, rawLimit, rawPage, userID, hasMetadata, m.db.PaginateFavorites)
}

func (m *PastaService) PaginateOnlyPublic(ctx context.Context, rawLimit, rawPage string, hasMetadata bool) (*dto.PaginatedPastaDTO, error) {
	adapted := func(ctx context.Context, limit int, offset int, _ int) ([]string, error) {
		return m.db.PaginateOnlyPublic(ctx, limit, offset)
	}
	return m.Paginate(ctx, rawLimit, rawPage, 0, hasMetadata, adapted)
}

func (m *PastaService) PaginateForUserByID(ctx context.Context, rawLimit, rawPage string, userID int, hasMetadata bool) (*dto.PaginatedPastaDTO, error) {
	return m.Paginate(ctx, rawLimit, rawPage, userID, hasMetadata, m.db.PaginateOnlyByUserID)
}

func mergeTextWithMetadata(texts []string, metadatas []models.Pasta, hasMetadata bool) []dto.TextsWithMetadata {
	result := make([]dto.TextsWithMetadata, 0, len(texts))

	for i := 0; i < len(texts); i++ {
		var metadata *models.Pasta
		if hasMetadata && i < len(metadatas) {
			metadata = &metadatas[i]
		} else {
			metadata = nil
		}
		result = append(result, dto.TextsWithMetadata{
			Text:     texts[i],
			Metadata: metadata,
		})
	}
	return result
}
