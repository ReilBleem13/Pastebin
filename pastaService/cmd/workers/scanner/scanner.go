package scanner

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	scannerrepo "pastebin/cmd/workers/scanner/repo"
	scannersrv "pastebin/cmd/workers/scanner/service"
	"pastebin/internal/infrastructure/kafka"
	"syscall"
	"time"

	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
	"github.com/theartofdevel/logging"
)

func StartScannerWorker(ctx context.Context, db *sqlx.DB, producer *kafka.Producer) {
	repo := scannerrepo.NewScannerDatabase(db)
	srv := scannersrv.NewScannerService(repo)

	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	go func() {
		<-quit
		logging.L(ctx).Info("Shitting down scanner worker")
		cancel()
	}()
	logging.L(ctx).Info("Cleaner worker is running")

	for {
		select {
		case <-ticker.C:
			expiredIDs, err := srv.GetExpiredPastas(ctx)
			if err != nil {
				logging.L(ctx).Error("Error getting expired pastas", logging.ErrAttr(err))
				continue
			}

			if len(expiredIDs) == 0 {
				logging.L(ctx).Info("Produces send ZERO pastas.")
				continue
			}

			sendCtx, sendCancel := context.WithTimeout(ctx, 10*time.Second)
			err = producer.Produce(sendCtx, kafka.CleanExpired{
				ObjectIDs: expiredIDs,
			})
			sendCancel()

			if err != nil {
				logging.L(ctx).Error("Failed to send kafka event", logging.ErrAttr(err))
			} else {
				logging.L(ctx).Info(fmt.Sprintf("Producer send %d pastas for delete", len(expiredIDs)))
			}
		case <-ctx.Done():
			logging.L(ctx).Info("Scanner worker stopped")
			return
		}
	}
}
