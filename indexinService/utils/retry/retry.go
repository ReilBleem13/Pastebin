package retry

import (
	"context"
	"errors"
	"log"
	"net"
	"time"

	"github.com/cenkalti/backoff/v4"
	"github.com/minio/minio-go/v7"
)

func IsRetryableErrorMinio(err error) bool {
	if err == nil {
		return false
	}

	var minioErr minio.ErrorResponse
	if errors.As(err, &minioErr) {
		switch minioErr.Code {
		case "SlowDown",
			"RequestTimeout",
			"InternalError",
			"ServiceUnavailable":
			return true
		}
	}

	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return true
	}

	return false
}

func WithRetry(ctx context.Context, operation func() error, isRetryable func(error) bool) error {
	expBackoff := backoff.NewExponentialBackOff()
	expBackoff.InitialInterval = 200 * time.Millisecond
	expBackoff.MaxInterval = 1 * time.Second
	expBackoff.MaxElapsedTime = 2 * time.Second
	expBackoff.RandomizationFactor = 0.3
	expBackoff.Multiplier = 2.0

	return backoff.RetryNotify(
		func() error {
			err := operation()
			if isRetryable(err) {
				return err
			}
			return backoff.Permanent(err)
		},
		backoff.WithContext(expBackoff, ctx),
		func(err error, duration time.Duration) {
			log.Printf("Retry after error: %v, waiting: %v\n", err, duration)
		},
	)
}
