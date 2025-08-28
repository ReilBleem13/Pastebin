package scanner

import (
	"context"
	"os"
	"os/signal"
	"pastebin/internal/service"
	"syscall"
	"time"

	"github.com/theartofdevel/logging"
)

func StartScannerWorker(ctx context.Context, srv *service.Service) {
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
			expiredIDs, err := srv.Pasta.GetExpiredPastas(ctx)
			if err != nil {
				logging.L(ctx).Error("Failed to get expired pastas", logging.ErrAttr(err))
				continue
			}

			if len(expiredIDs) == 0 {
				logging.L(ctx).Info("Produces send ZERO pastas.")
				continue
			}

			if err := srv.Pasta.DeletePastas(ctx, expiredIDs); err != nil {
				logging.L(ctx).Error("Failed to delete expired pastas", logging.ErrAttr(err))
				continue
			} else {
				logging.L(ctx).Info("Count of deleted pastas", logging.IntAttr("ids", len(expiredIDs)))
			}

		case <-ctx.Done():
			logging.L(ctx).Info("Scanner worker stopped")
			return
		}
	}
}
