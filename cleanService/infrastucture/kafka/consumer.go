package kafka

import (
	"cleanService/config"
	"context"
	"fmt"

	"github.com/confluentinc/confluent-kafka-go/kafka"
	"github.com/theartofdevel/logging"
)

type Handler interface {
	DeleteExpiredPastas(message []byte) error
}

type Consumer struct {
	consumer *kafka.Consumer
	logger   *logging.Logger
	handler  Handler
	stop     chan struct{}
	workers  int
}

func NewConsumer(ctx context.Context, handler Handler, cfg config.KafkaConfig, workers int) (*Consumer, error) {
	config := &kafka.ConfigMap{
		"bootstrap.servers":        cfg.Address,
		"group.id":                 cfg.Group,
		"auto.offset.reset":        cfg.AutoOffsetReset,
		"enable.auto.commit":       cfg.EnableAutoCommit,
		"isolation.level":          cfg.IsolationLevel,
		"enable.partition.eof":     cfg.EnablePartitionEof,
		"go.events.channel.enable": cfg.GoEventsChannelEnable,
	}
	c, err := kafka.NewConsumer(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create consumer: %w", err)
	}

	err = c.Subscribe(cfg.Topic, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to subscrive: %w", err)
	}
	return &Consumer{
		consumer: c,
		logger:   logging.L(ctx),
		stop:     make(chan struct{}),
		handler:  handler,
		workers:  workers,
	}, nil
}

func (c *Consumer) Start() {
	msgCh := make(chan *kafka.Message, 1000)

	for i := 0; i < c.workers; i++ {
		go func() {
			for msg := range msgCh {
				if err := c.handler.DeleteExpiredPastas(msg.Value); err != nil {
					c.logger.Error("handler error", logging.ErrAttr(err))
					continue
				}

				_, err := c.consumer.CommitMessage(msg)
				if err != nil {
					c.logger.Error("commit error", logging.ErrAttr(err))
				}
			}

		}()
	}

	for {
		select {
		case <-c.stop:
			close(msgCh)
			return
		default:
			msg, err := c.consumer.ReadMessage(-1)
			if err != nil {
				c.logger.Error("kafka read error", logging.ErrAttr(err))
				continue
			}
			msgCh <- msg
		}
	}
}

func (c *Consumer) Stop() {
	close(c.stop)
	c.consumer.Close()
}
