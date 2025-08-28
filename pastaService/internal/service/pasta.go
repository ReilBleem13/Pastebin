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
	"pastebin/pkg/retry"
	"pastebin/pkg/validate"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/theartofdevel/logging"
	"golang.org/x/sync/errgroup"
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

	link string = "localhost:10002/receive/"

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

	m.logger.Debug("Validating request create pasta")
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

	passwordHash := ""
	if req.Password != "" {
		m.logger.Debug("hashing password")

		passwordHash, err = utils.HashPassword(req.Password)
		if err != nil {
			return nil, fmt.Errorf("failed to hash password: %w", err)
		}
	}

	data := []byte(req.Message)

	m.logger.Debug("Store to s3")
	pastaMetadata, err := m.s3.Store(ctx, prefix, data)
	if err != nil {
		return nil, fmt.Errorf("failed to store pasta in S3: %w", err)
	}

	pastaMetadata.Hash = utils.Hash(pastaMetadata.ObjectID)
	pastaMetadata.ExpiresAt = expiresAt
	pastaMetadata.ExpireAfterRead = req.ExpireAfterRead
	pastaMetadata.PasswordHash = passwordHash
	pastaMetadata.Language = req.Language
	pastaMetadata.Visibility = models.Visibility(req.Visibility)
	pastaMetadata.UserID = userID
	pastaMetadata.CreatedAt = timeNow

	m.logger.Debug("Store to database")
	err = retry.Retry(ctx, func() error {
		return m.db.Create(ctx, pastaMetadata)
	}, retry.IsRetryableErrorDatabase, retry.NewConfigWithComponent("db"), m.logger)
	if err != nil {
		if deleteErr := m.s3.Delete(ctx, pastaMetadata.ObjectID); deleteErr != nil {
			m.logger.Error("Failed to cleanup S3 file after DB. Error:", logging.ErrAttr(deleteErr), logging.StringAttr("hash", pastaMetadata.Hash))
		}
		return nil, fmt.Errorf("failed to create new pasta in database: %w", err)
	}

	m.logger.Debug("Indexing")
	if err := m.elastic.Indexing(data, pastaMetadata.ObjectID); err != nil {
		return nil, fmt.Errorf("failed to index pasta: %w", err)
	}

	wg := &sync.WaitGroup{}

	wg.Add(1)
	go func() {
		defer wg.Done()
		m.logger.Debug("Add text to cache")
		if err := m.cache.AddText(ctx, pastaMetadata.Hash, data); err != nil {
			m.logger.Error("Failed to cache pasta. Error:", logging.ErrAttr(err), logging.StringAttr("hash", pastaMetadata.Hash))
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		m.logger.Debug("Creating views")
		if err := m.cache.CreateViews(ctx, pastaMetadata.Hash, &expiration); err != nil {
			m.logger.Error("Failed to add views. Error:", logging.ErrAttr(err), logging.StringAttr("hash", pastaMetadata.Hash))
		}
	}()
	wg.Wait()
	return pastaMetadata, nil
}

/*
1. Проверка существует ли паста.
2. Если паста приватная или флаг forAccessCheck = true, проверяется имеет ли пользователь право на эту пасту.
3. Получение хеша пароля, если не пустой, то проверка.
*/
func (m *PastaService) Permission(ctx context.Context, hash, password, visibility string, userID int, forAccessCheck bool) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	m.logger.Debug("Cheking is pasta exists")
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

	if visibility == string(models.VisibilityPrivate) || forAccessCheck {
		m.logger.Debug("Checking access permissions")

		var hasAccess bool
		err = retry.Retry(ctx, func() error {
			var err error
			hasAccess, err = m.db.IsAccessPermission(ctx, userID, hash)
			return err
		}, retry.IsRetryableErrorDatabase, retry.NewConfigWithComponent("db"), m.logger)
		if err != nil {
			return fmt.Errorf("failed to check access private: %w", err)
		}
		if !hasAccess {
			return customerrors.ErrNoAccess
		}
		return nil
	}

	m.logger.Debug("Getting hash password")
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

	m.logger.Debug("Deleting from db")
	var objectID string
	err := retry.Retry(ctx, func() error {
		var err error
		objectID, err = m.db.DeleteMetadata(ctx, hash)
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
		m.logger.Debug("Deleting from s3")

		if err := m.s3.Delete(ctx, objectID); err != nil {
			errChan <- fmt.Errorf("failed to delete pasta from S3: %w", err)
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		m.logger.Debug("Deleting from elastic")

		if err := m.elastic.DeleteDocument(ctx, objectID); err != nil {
			errChan <- fmt.Errorf("failed to delete pasta from elastic: %w", err)
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		m.logger.Debug("Deleting views, meta and text")

		if err := m.cache.DeleteAll(ctx, hash); err != nil {
			m.logger.Error("failed to delete all about pasta from cache. Error:", logging.ErrAttr(err), logging.StringAttr("hash", hash))
		}
	}()

	wg.Wait()
	close(errChan)

	var errs []error
	for err := range errChan {
		if err != nil {
			errs = append(errs, err)
		}
	}

	if len(errs) > 0 {
		m.logger.Error("Some delete steps failed.", logging.AnyAttr("Errors", errs))
		return errs[0]
	}
	return nil
}

func (m *PastaService) GetText(ctx context.Context, keyText, objectID, hash string) (string, error) {
	m.logger.Debug("Getting text from cache")
	text, err := m.cache.GetText(ctx, keyText)
	if err == nil {
		return text, nil
	}

	if !errors.Is(err, customerrors.ErrKeyDoesntExist) {
		m.logger.Warn("Failed to get text from cache (fallback to s3). Error:", logging.ErrAttr(err), logging.StringAttr("hash", hash))
	}

	m.logger.Debug("Getting text from minio")
	text, err = m.s3.Get(ctx, objectID)
	if err != nil {
		return "", fmt.Errorf("failed to get file from minio: %w", err)
	}

	m.logger.Debug("Adding text to cache")
	if err := m.cache.AddText(ctx, hash, []byte(text)); err != nil {
		m.logger.Error("Failed to add text to cache. Error:", logging.ErrAttr(err), logging.StringAttr("hash", hash))
	}
	return text, nil
}

func (m *PastaService) GetMetadata(ctx context.Context, keyMeta string, hash string) (*models.Pasta, error) {
	m.logger.Debug("Getting metadata from cache")
	metadata, err := m.cache.GetMeta(ctx, keyMeta)
	if err == nil {
		return metadata, err
	}

	if !errors.Is(err, customerrors.ErrKeyDoesntExist) {
		m.logger.Warn("Failed to get metadata from cache (falling back to DB). Error:", logging.ErrAttr(err), logging.StringAttr("hash", hash))
	}

	m.logger.Debug("Getting metadata from db")
	err = retry.Retry(ctx, func() error {
		metadata, err = m.db.GetMetadata(ctx, hash)
		return err
	}, retry.IsRetryableErrorDatabase, retry.NewConfigWithComponent("db"), m.logger)
	if err != nil {
		return nil, err
	}

	m.logger.Debug("Adding metadata to cache")
	if err := m.cache.AddMeta(ctx, metadata); err != nil {
		m.logger.Error("Failed to add metadata to cache. Error:", logging.ErrAttr(err), logging.StringAttr("hash", hash))
	}
	return metadata, nil
}

func (m *PastaService) Get(ctx context.Context, hash string, withMetadata bool, addView bool) (*models.PastaWithData, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	var result models.PastaWithData
	var metadata *models.Pasta
	var views int

	m.logger.Debug("Getting objectID")
	objectID, err := m.db.GetKey(ctx, hash)
	if err != nil {
		return nil, fmt.Errorf("failed to get objectID: %w", err)
	}

	keyMeta := fmt.Sprintf("%s:%s", metaPrefix, hash)
	keyText := fmt.Sprintf("%s:%s", textPrefix, hash)

	m.logger.Debug("Getting text")
	text, err := m.GetText(ctx, keyText, objectID, hash)
	if err != nil {
		return nil, fmt.Errorf("failed to get text: %w", err)
	}

	if addView {
		m.logger.Debug("Incrementing view (addView = true)")
		views, err = m.cache.IncrViews(ctx, hash)
		if err != nil {
			m.logger.Error("Failed to increment views. Error:", logging.ErrAttr(err), logging.StringAttr("hash", hash))
			views = -1
		}
	} else {
		m.logger.Debug("Incrementing view (addView = false)")
		viewsString, err := m.cache.GetViews(ctx, hash)
		if err != nil {
			m.logger.Error("Failed to increment views. Error:", logging.ErrAttr(err), logging.StringAttr("hash", hash))
			views = -1
		}
		viewsInt, err := strconv.Atoi(viewsString)
		if err != nil {
			m.logger.Debug("failed convert string to int")
		}
		views = viewsInt
	}

	result.Text = text
	if withMetadata {
		m.logger.Debug("Getting metadata")

		var err error
		metadata, err = m.GetMetadata(ctx, keyMeta, hash)
		if err != nil {
			return nil, fmt.Errorf("failed to get metadata: %w", err)
		}
		result.Metadata = metadata
		result.Metadata.Views = views
	}

	var isExpireAfterRead bool

	if metadata != nil {
		isExpireAfterRead = metadata.ExpireAfterRead
	} else {
		m.logger.Debug("Getting expire after read field")
		isExpireAfterRead, err = m.db.GetExpireAfterReadField(ctx, hash)
		if err != nil {
			return nil, fmt.Errorf("failed to get expire-after-read flag: %w", err)
		}
	}

	if isExpireAfterRead {
		m.logger.Debug("Expire after read. Deleting")
		if err = m.Delete(ctx, hash); err != nil {
			return nil, fmt.Errorf("failed to delete pasta after read: %w", err)
		}
	}
	return &result, nil
}

func (m *PastaService) Search(ctx context.Context, word string) ([]string, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	m.logger.Debug("Search word in elastic")
	objectIDs, err := m.elastic.SearchWord(word)
	if err != nil {
		return nil, fmt.Errorf("failed to search word: %w", err)
	}
	m.logger.Debug("Searched objectIDs", logging.AnyAttr("ObjectIDs", objectIDs))
	if len(objectIDs) == 0 {
		return []string{}, customerrors.ErrEmptySearchResult
	}

	m.logger.Debug("Get public hashs")
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

	m.logger.Debug("Getting objectID from db")
	var objectID string
	err := retry.Retry(ctx, func() error {
		var err error
		objectID, err = m.db.GetKey(ctx, hash)
		return err
	}, retry.IsRetryableErrorDatabase, retry.NewConfigWithComponent("db"), m.logger)
	if err != nil {
		return nil, fmt.Errorf("failed to get objectID from db: %w, hash=%s", err, hash)
	}

	type updateResult struct {
		metadata *models.Pasta
		name     string
		err      error
	}

	resultCh := make(chan updateResult, 5)
	metaCh := make(chan *models.Pasta, 1)
	metadata := &models.Pasta{}
	wg := &sync.WaitGroup{}

	wg.Add(1)
	go func() {
		defer wg.Done()
		m.logger.Debug("Update s3")

		reader := bytes.NewReader(newText)

		if err := m.s3.Update(ctx, reader, objectID); err != nil {
			resultCh <- updateResult{name: "s3", err: err}
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		m.logger.Debug("Update db")

		var err error
		err = retry.Retry(ctx, func() error {
			metadata, err = m.db.UpdateSizeAndReturnAll(ctx, hash, len(newText))
			return err
		}, retry.IsRetryableErrorDatabase, retry.NewConfigWithComponent("db"), m.logger)
		if metadata != nil {
			metaCh <- metadata
		}
		resultCh <- updateResult{name: "db", metadata: metadata, err: err}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		m.logger.Debug("Update cache - text")

		if err := m.cache.AddText(ctx, hash, newText); err != nil {
			resultCh <- updateResult{name: "cache-text", err: err}
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()

		select {
		case metadata := <-metaCh:
			m.logger.Debug("Update cache - meta")

			if err := m.cache.AddMeta(ctx, metadata); err != nil {
				resultCh <- updateResult{name: "cache-metadata", err: err}
			}

		case <-ctx.Done():
			resultCh <- updateResult{name: "cache-metadata", err: fmt.Errorf("context timeout before metadata received")}
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		m.logger.Debug("Update ElasticSearch")

		if err := m.elastic.Indexing(newText, objectID); err != nil {
			resultCh <- updateResult{name: "elastic-update", err: err}
		}
	}()

	go func() {
		wg.Wait()
		close(resultCh)
		close(metaCh)
	}()

	var firstCriticalErr error
	var finalMetadata *models.Pasta

loop:
	for {
		select {
		case res, ok := <-resultCh:
			if !ok {
				break loop
			}
			if res.err != nil {
				if strings.HasPrefix(res.name, "cache") {
					m.logger.Error("Update failed", logging.StringAttr("field", fmt.Sprintf("%s update failed", res.name)), logging.ErrAttr(res.err))
				} else {
					m.logger.Error(fmt.Sprintf("%s update failed", res.name), logging.ErrAttr(res.err))
					if firstCriticalErr == nil {
						firstCriticalErr = res.err
					}
				}
			}
			if res.name == "db" && res.metadata != nil {
				finalMetadata = res.metadata
			}
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}

	if firstCriticalErr != nil {
		return nil, firstCriticalErr
	}
	return finalMetadata, nil
}

func (m *PastaService) Favorite(ctx context.Context, hash string, userID int) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	visibility, err := m.db.GetVisibility(ctx, hash)
	if err != nil {
		return fmt.Errorf("failed get visibility: %w", err)
	}

	if visibility == string(models.VisibilityPrivate) {
		return customerrors.ErrNotAllowed
	}

	m.logger.Debug("Checking is pasta exists")
	var exists bool
	err = retry.Retry(ctx, func() error {
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
	m.logger.Debug("Getting favorite")
	return m.db.Favorite(ctx, hash, userID)
}

func (m *PastaService) GetFavorite(ctx context.Context, userID, favoriteID int, withMetadata bool) (*models.PastaWithData, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	m.logger.Debug("Getting favorite and cheaking user")
	var hash string
	err := retry.Retry(ctx, func() error {
		var err error
		hash, err = m.db.GetFavoriteAndCheckUser(ctx, userID, favoriteID)
		return err
	}, retry.IsRetryableErrorDatabase, retry.NewConfigWithComponent("db"), m.logger)
	if err != nil {
		return nil, fmt.Errorf("failed to get favorite and check user: %w", err)
	}

	m.logger.Debug("Getting text and metadata(optional)")
	pasta, err := m.Get(ctx, hash, withMetadata, false)
	if err != nil {
		return nil, fmt.Errorf("failed to get pasta: %w, hash=%s", err, hash)
	}
	return pasta, nil
}

func (m *PastaService) DeleteFavorite(ctx context.Context, userID, favoriteID int) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	m.logger.Debug("Delete favorite")
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
		limit, err = strconv.Atoi(rawLimit)
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

	m.logger.Debug("Paginate objectIDs from database")
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

	metadatas := make([]models.Pasta, len(objectIDs))

	var metaErr error

	wg := &sync.WaitGroup{}
	if hasMetadata {
		wg.Add(1)
		go func() {
			defer wg.Done()

			m.logger.Debug("Getting many metadata")
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

			m.logger.Debug("Getting views")
			for i := range metadatas {
				viewsStr, err := m.cache.GetViews(ctx, metadatas[i].Hash)
				if err != nil {
					m.logger.Error("failed to get views. Error:", logging.StringAttr("hash", metadatas[i].Hash))
					metadatas[i].Views = -1
					continue
				}

				viewsInt, err := strconv.Atoi(viewsStr)
				if err != nil {
					m.logger.Error("failed to convert views. Error:", logging.StringAttr("hash", metadatas[i].Hash))
					metadatas[i].Views = -1
					continue
				}
				metadatas[i].Views = viewsInt
			}

		}()
	}

	m.logger.Debug("Getting text from s3")
	texts, err := m.s3.GetFiles(ctx, objectIDs)
	if err != nil {
		return nil, fmt.Errorf("failed to get files from s3: %w", err)
	}
	wg.Wait()

	if metaErr != nil {
		return nil, fmt.Errorf("failed to get metadata: %w", metaErr)
	}

	m.logger.Debug("Merge text with metadata")
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

func (m *PastaService) GetExpiredPastas(ctx context.Context) ([]string, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	objectIDs, err := m.db.GetExpiredPastas(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get objectIDs: %w", err)
	}
	return objectIDs, nil
}

func (m *PastaService) DeletePastas(ctx context.Context, objectIDs []string) error {
	g, ctx := errgroup.WithContext(ctx)

	g.Go(func() error {
		if err := m.db.DeletePastas(ctx, objectIDs); err != nil {
			return fmt.Errorf("failed to delete from db: %w", err)
		}
		return nil
	})

	g.Go(func() error {
		if err := m.s3.DeleteMany(ctx, objectIDs); err != nil {
			return fmt.Errorf("failed to delete from s3: %w", err)
		}
		return nil
	})

	g.Go(func() error {
		if err := m.elastic.DeleteManyDocument(ctx, objectIDs); err != nil {
			return fmt.Errorf("failed to delete from elastic: %w", err)
		}
		return nil
	})
	return g.Wait()
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
