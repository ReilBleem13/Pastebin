package scanner

import (
	"context"
	"os"
	"os/signal"
	scannerrepo "pastebin/cmd/workers/scanner/repo"
	scannersrv "pastebin/cmd/workers/scanner/service"
	mykafka "pastebin/internal/infrastructure/kafka"
	"pastebin/pkg/logging"
	"syscall"
	"time"

	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
)

func StartScannerWorker(ctx context.Context, db *sqlx.DB, logger *logging.Logger, kafkaAddress string) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	repo := scannerrepo.NewScannerDatabase(db)
	srv := scannersrv.NewScannerService(repo, logger)

	producer, err := mykafka.NewProducer(kafkaAddress)
	if err != nil {
		logger.Fatalf("failed to create kafka prodcer: %v", err)
	}
	logger.Info("Producer was created")

	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-quit
		logger.Info("shutting down scanner worker...")
		cancel()
	}()
	logger.Infof("Cleaner worker is running...")

	for {
		select {
		case <-ticker.C:
			expiredIDs, err := srv.GetExpiredPastas(ctx)
			if err != nil {
				logger.Errorf("error getting epired pastas: %v", err)
				continue
			}

			if len(expiredIDs) == 0 {
				continue
			}

			messages := make([]string, 0, len(expiredIDs))
			messages = append(messages, expiredIDs...)

			sendCtx, sendCancel := context.WithTimeout(ctx, 10*time.Second)
			defer sendCancel()

			err = producer.Produce(sendCtx, mykafka.CleanExpired{
				ObjectIDs: messages,
			})
			if err != nil {
				logger.Errorf("failed to send kafka event: %v", err)
			} else {
				logger.Infof("Producer send %d pastas for delete", len(messages))
			}
		case <-ctx.Done():
			return
		}
	}
}
