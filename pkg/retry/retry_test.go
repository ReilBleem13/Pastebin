package retry

import (
	"context"
	"errors"
	"net"
	"testing"

	"github.com/lib/pq"
	"github.com/minio/minio-go/v7"
)

func TestIsRetryableErrorDatabase(t *testing.T) {
	pgErr := &pq.Error{Code: "40001"}
	if !IsRetryableErrorDatabase(pgErr) {
		t.Error("Should retry on serialization_failure")
	}

	netErr := &net.DNSError{IsTimeout: true}
	if !IsRetryableErrorDatabase(netErr) {
		t.Error("Should retry on network timeout")
	}

	otherErr := errors.New("some error")
	if IsRetryableErrorDatabase(otherErr) {
		t.Error("Should not retry on generic error")
	}
}

func TestIsRetryableErrorMinio(t *testing.T) {
	// Временная ошибка MinIO
	minioErr := minio.ErrorResponse{Code: "SlowDown"}
	if !IsRetryableErrorMinio(minioErr) {
		t.Error("Should retry on SlowDown")
	}

	// Сетевой таймаут
	netErr := &net.DNSError{IsTimeout: true}
	if !IsRetryableErrorMinio(netErr) {
		t.Error("Should retry on network timeout")
	}

	// Не временная ошибка
	otherErr := errors.New("some error")
	if IsRetryableErrorMinio(otherErr) {
		t.Error("Should not retry on generic error")
	}
}

func TestIsRetryableErrorRedis(t *testing.T) {
	netErr := &net.DNSError{IsTimeout: true}
	if !IsRetryableErrorRedis(netErr) {
		t.Error("Should retry on network timeout")
	}

	otherErr := errors.New("some error")
	if IsRetryableErrorRedis(otherErr) {
		t.Error("Should not retry on generic error")
	}
}

func TestWithRetry(t *testing.T) {
	attempts := 0
	maxAttempts := 3
	err := WithRetry(context.Background(), func() error {
		attempts++
		if attempts < maxAttempts {
			return &pq.Error{Code: "40001"}
		}
		return nil
	}, IsRetryableErrorDatabase)
	if err != nil {
		t.Errorf("WithRetry should succeed, got error: %v", err)
	}
	if attempts != maxAttempts {
		t.Errorf("WithRetry should attempt %d times, got %d", maxAttempts, attempts)
	}
}
