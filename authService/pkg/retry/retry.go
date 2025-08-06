package retry

import (
	"context"
	"time"

	"github.com/cenkalti/backoff/v4"
	"github.com/theartofdevel/logging"
)

type RetryFunc func() error

type Config struct {
	InitialInterval     time.Duration
	MaxInterval         time.Duration
	MaxElapsedTime      time.Duration
	RandomizationFactor float64
	Component           string
}

var DefaultConfig = Config{
	InitialInterval:     200 * time.Millisecond,
	MaxInterval:         1 * time.Second,
	MaxElapsedTime:      2 * time.Second,
	RandomizationFactor: 0.3,
	Component:           "unknown",
}

func Retry(ctx context.Context, fn RetryFunc, isRetryable func(error) bool, cfg Config, logger *logging.Logger) error {
	bo := backoff.NewExponentialBackOff()
	bo.InitialInterval = cfg.InitialInterval
	bo.MaxInterval = cfg.MaxInterval
	bo.MaxElapsedTime = cfg.MaxElapsedTime
	bo.RandomizationFactor = cfg.RandomizationFactor
	bo.Reset()

	backoffCtx := backoff.WithContext(bo, ctx)

	var attempt int

	return backoff.RetryNotify(
		func() error {
			attempt++
			err := fn()
			if err == nil {
				return nil
			}
			if isRetryable(err) {
				return err
			}
			return backoff.Permanent(err)
		},
		backoffCtx,
		func(err error, delay time.Duration) {
			logger.Debug("[retry]", logging.StringAttr("[component]", cfg.Component), logging.IntAttr("attempt", attempt), logging.ErrAttr(err), logging.StringAttr("delay", delay.String()))
		},
	)
}

func NewConfigWithComponent(component string) Config {
	cfg := DefaultConfig
	cfg.Component = component
	return cfg
}
