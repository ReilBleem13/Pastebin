package service

import (
	"context"
	"errors"
	"fmt"
	"log"
	"pastebin/internal/domain"
	customerrors "pastebin/internal/errors"
	"pastebin/internal/models"
	"pastebin/internal/repository"
	"pastebin/internal/utils"
	"pastebin/pkg/dto"
	hashing "pastebin/pkg/hash"
	"pastebin/pkg/validate"
)

type PastaService struct {
	s3      domain.S3
	cache   domain.PastaCache
	db      domain.PastaDatabase
	elastic domain.Elastic
}

const (
	defaultNewFilePrefix        = "public:"
	indexForElastic      string = "pastas" // добавить в структуру конфиг
	visibilityIsPrivate         = "private"
)

func NewPastaService(repo *repository.Repository) *PastaService {
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
			return nil, err
		}
	}

	data := []byte(req.Message)
	pastaMetadata, err := m.s3.StoreFile(ctx, prefix, &data, options)
	if err != nil {
		return nil, err
	}
	pastaMetadata.Hash = hashing.Hash(pastaMetadata.Key)
	pastaMetadata.ExpiresAt = timeExpiration
	pastaMetadata.PasswordHash = passwordHash
	pastaMetadata.Language = req.Language
	pastaMetadata.Visibility = req.Visibility
	pastaMetadata.UserID = userID

	// добавление метаданных в бд
	if err := m.db.CreateMetadata(ctx, pastaMetadata); err != nil {
		return nil, err
	}

	// кеширование текста
	if err := m.cache.AddText(ctx, pastaMetadata.Hash, data); err != nil {
		return nil, err
	}

	// индексирование текста
	if err := m.elastic.NewIndex(&data, pastaMetadata.Key, indexForElastic, nil); err != nil {
		return nil, err
	}
	return pastaMetadata, nil
}

func (m *PastaService) Permission(ctx context.Context, hash, password, visibility string, userID int) error {
	// проверяем существования пасты
	pastaExists, err := m.db.IsPastaExists(ctx, hash)
	if err != nil {
		return err
	}

	if !pastaExists {
		return customerrors.ErrPastaNotFound
	}

	// проверка, если паста приватная
	if visibility == visibilityIsPrivate {
		exists, err := m.db.IsAccessPrivate(ctx, userID, hash)
		if err != nil {
			return err
		}
		if !exists {
			return customerrors.ErrNoAccess
		}
		return nil
	}

	// проверка, если паста публичная
	passwordHash, err := m.db.GetPassword(ctx, hash)
	log.Printf("password: %s", passwordHash)
	log.Printf("password me: %s", password)

	if err != nil {
		return err
	}
	if passwordHash != "" {
		if !utils.CheckPasswordHash(password, passwordHash) {
			log.Println(utils.CheckPasswordHash(password, passwordHash))
			return customerrors.ErrWrongPassword
		}
		return nil
	}
	return nil
}

func (m *PastaService) GetText(ctx context.Context, keyText, objectID, hash string) (*string, error) {
	text, err := m.cache.GetText(ctx, keyText)
	if err != nil {
		if errors.Is(err, customerrors.ErrKeyDoesntExist) {
			text, err = m.s3.GetFile(ctx, objectID)
			if err != nil {
				return nil, err
			}
			log.Println("текст из MINIO")

			if err := m.cache.AddText(ctx, hash, []byte(*text)); err != nil {
				return nil, err
			}
			log.Println("текст добавлен в Redis")
		} else {
			return nil, err
		}
	}
	return text, nil
}

func (m *PastaService) Get(ctx context.Context, hash string, flag bool) (*models.PastaWithData, error) {
	result := &models.PastaWithData{}

	objectID, err := m.db.GetKey(ctx, hash)
	if err != nil {
		return nil, err
	}

	keyMeta := fmt.Sprintf("meta:%s", hash)
	keyText := fmt.Sprintf("data:%s", hash)

	if !flag {
		text, err := m.GetText(ctx, keyText, objectID, hash)
		if err != nil {
			return nil, err
		}
		// только инкримент просмотров
		_, err = m.cache.Views(ctx, hash)
		if err != nil {
			return nil, err
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
		if errors.Is(err, customerrors.ErrKeyDoesntExist) {
			metadata, err = m.db.GetMetadata(ctx, objectID)
			if err != nil {
				metaErrCh <- err
				return
			}
			log.Println("Метаданные из DB")

			if err := m.cache.AddMeta(ctx, metadata); err != nil {
				metaErrCh <- err
				return
			}
			log.Println("Метаданные добавлены в Redis")
		} else if err != nil {
			metaErrCh <- err
			return
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
		return nil, err
	}
	result.Metadata = metadata
	result.Metadata.Views = views

	result.Text = *text
	return result, nil
}

// func (m *MinioService) GetMany(ctx context.Context, objectIDs []string) ([]string, error) {
// 	return m.client.GetFiles(ctx, objectIDs)
// }

// func (m *MinioService) DeleteOne(ctx context.Context, hash string) error {
// 	key, err := m.repo.DeleteMetadata(ctx, hash)
// 	if err != nil {
// 		return err
// 	}
// 	return m.client.DeleteFile(ctx, key)
// }

// func (m *MinioService) DeleteMany(ctx context.Context, objectIDs []string) error {
// 	return m.client.DeleteFiles(ctx, objectIDs)
// }

// func (m *MinioService) Paginate(ctx context.Context, maxKeys, startAfter string, userID *int) ([]models.PastaPaginated, string, error) {
// 	var prefix string
// 	var maxKeysInt int

// 	if maxKeys != "" {
// 		var err error
// 		maxKeysInt, err = strconv.Atoi(maxKeys)
// 		if err != nil {
// 			return []models.PastaPaginated{}, "", fmt.Errorf("invalid maxkeys format: %v", err)
// 		}

// 		if maxKeysInt < 5 {
// 			maxKeysInt = 5
// 		}
// 	} else {
// 		maxKeysInt = 5
// 	}

// 	if userID != nil {
// 		prefix = fmt.Sprintf("user:%d", *userID)
// 		pastas, nextKey, err := m.client.PaginateFilesByUserID(ctx, maxKeysInt, startAfter, prefix)
// 		if err != nil {
// 			return []models.PastaPaginated{}, "", err
// 		}
// 		responses := formResponse(pastas)
// 		return responses, nextKey, nil
// 	} else {
// 		prefix = "public"
// 	}

// 	pastas, nextKey, err := m.client.PaginateFiles(ctx, maxKeysInt, startAfter, prefix)
// 	if err != nil {
// 		return []models.PastaPaginated{}, "", err
// 	}

// 	responses := formResponse(pastas)
// 	return responses, nextKey, nil
// }

// func formResponse(pastas []string) []models.PastaPaginated {
// 	var responses []models.PastaPaginated

// 	for i, pasta := range pastas {
// 		response := models.PastaPaginated{
// 			Number: i + 1,
// 			Pasta:  pasta,
// 		}
// 		responses = append(responses, response)
// 	}
// 	return responses
// }

// func prtSrt(s string) *string {
// 	return &s
// }
