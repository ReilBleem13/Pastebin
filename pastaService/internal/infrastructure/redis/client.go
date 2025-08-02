package redis

import (
	"context"
	"fmt"
	"net"
	"pastebin/internal/config"
	"strings"

	"github.com/redis/go-redis/v9"
	"github.com/theartofdevel/logging"
)

type RedisClient struct {
	Client redis.UniversalClient
}

type LoggingHook struct {
	logger *logging.Logger
}

func (l LoggingHook) DialHook(next redis.DialHook) redis.DialHook {
	return func(ctx context.Context, network, addr string) (net.Conn, error) {
		l.logger.Debug("[Redis] Dialing.", logging.StringAttr("network", network),
			logging.StringAttr("addr", addr))
		return next(ctx, network, addr)
	}
}

func (l LoggingHook) ProcessHook(next redis.ProcessHook) redis.ProcessHook {
	return func(ctx context.Context, cmd redis.Cmder) error {
		l.logger.Debug("[Redis] -> ", logging.StringAttr("CMD", cmd.Name()),
			logging.StringAttr("ARGS", ArgsToString(cmd.Args())))
		err := next(ctx, cmd)
		l.logger.Debug("[Redis] <- ", logging.StringAttr("DONE", cmd.Name()),
			logging.ErrAttr(cmd.Err()))
		return err
	}
}

func (l LoggingHook) ProcessPipelineHook(next redis.ProcessPipelineHook) redis.ProcessPipelineHook {
	return func(ctx context.Context, cmds []redis.Cmder) error {
		l.logger.Debug("[Redis] -> ", logging.IntAttr("Pipeline", len(cmds)))
		for _, cmd := range cmds {
			l.logger.Debug(" -> ", logging.StringAttr("CMD", cmd.Name()),
				logging.StringAttr("ARGS", ArgsToString(cmd.Args())))
		}

		err := next(ctx, cmds)

		for _, cmd := range cmds {
			if cmd.Err() != nil {
				l.logger.Debug(" <- ", logging.StringAttr("CMD", cmd.Name()),
					logging.StringAttr("ERR", cmd.Err().Error()))
			} else {
				l.logger.Debug(" <- ", logging.StringAttr("CMD", cmd.Name()),
					logging.StringAttr("ERR", "nil"))
			}
		}

		return err
	}
}

func NewRedisClient(ctx context.Context, cfg config.RedisConfig) *RedisClient {
	var client redis.UniversalClient

	if cfg.Mode == "cluster" {
		client = redis.NewClusterClient(&redis.ClusterOptions{
			Addrs:        cfg.Addrs,
			Password:     cfg.Password,
			DialTimeout:  cfg.DialTimeout,
			ReadTimeout:  cfg.ReadTimeout,
			WriteTimeout: cfg.WriteTimeout,
			PoolSize:     cfg.PoolSize,
			MaxRetries:   cfg.MaxRetries,
		})
	} else {
		client = redis.NewClient(&redis.Options{
			Addr:         cfg.Addr,
			Password:     cfg.Password,
			DialTimeout:  cfg.DialTimeout,
			ReadTimeout:  cfg.ReadTimeout,
			WriteTimeout: cfg.WriteTimeout,
			PoolSize:     cfg.PoolSize,
			MaxRetries:   cfg.MaxRetries,
		})
	}

	client.AddHook(LoggingHook{
		logger: logging.L(ctx),
	})

	return &RedisClient{Client: client}
}

func ArgsToString(args []interface{}) string {
	var parts []string
	for _, arg := range args {
		parts = append(parts, fmt.Sprint(arg))
	}
	return strings.Join(parts, ", ")
}
