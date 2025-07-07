package domain

import (
	"context"
	"pastebin/internal/infrastructure/kafka"
)

type AtLeastOnceProducer interface {
	Produce(ctx context.Context, event kafka.Event) error
}
