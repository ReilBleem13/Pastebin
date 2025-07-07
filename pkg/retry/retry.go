package retry

import (
	"context"
	"errors"
	"log"
	"net"
	"time"

	"github.com/cenkalti/backoff/v4"
	"github.com/lib/pq"
	"github.com/minio/minio-go/v7"
)

func IsRetryableErrorDatabase(err error) bool {
	if err == nil {
		return false
	}
	var pqErr *pq.Error
	if errors.As(err, &pqErr) {
		switch pqErr.Code {
		case "40001", // serialization_failure
			"40P01", // deadlock_detected
			"53300", // too_many_connections
			"57P03", // cannot_connect_now
			"08006", // connection_failure
			"08001", // sqlclient_unable_to_establish_sqlconnection
			"08003", // connection_does_not_exist
			"08000", // connection_exception
			"08004", // sqlserver_rejected_establishment_of_sqlconnection
			"08007": // transaction_resolution_unknown
			return true
		}
	}

	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return true
	}

	return false
}

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

func IsRetryableErrorRedis(err error) bool {
	if err == nil {
		return false
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
