package service

// type ExpiredDeleteService struct {
// 	repo  domain.PastaDatabase
// 	minio domain.S3
// }

// func NewExpiredDeleteService(repo domain.PastaDatabase, minio domain.S3) *ExpiredDeleteService {
// 	return &ExpiredDeleteService{repo: repo, minio: minio}
// }

// func (s *ExpiredDeleteService) CleanupExpiredPasta(ctx context.Context) error {
// 	expiredPasta, err := s.repo.GetKeysExpiredPasta(ctx)
// 	if err != nil {
// 		return err
// 	}

// 	log.Println("Удаление в minio")
// 	if err := s.minio.DeleteFiles(ctx, expiredPasta); err != nil {
// 		log.Printf("Failed to delete expired pasta from MinIO: %v", err)
// 	}

// 	log.Println("Удаление в db")
// 	if err := s.repo.DeleteExpiredPasta(ctx, expiredPasta); err != nil {
// 		log.Printf("Failed to delete expired pasta from DB: %v", err)
// 	}

// 	log.Printf("Удаление завершено. Удалено %d паст", len(expiredPasta))
// 	return nil
// }
