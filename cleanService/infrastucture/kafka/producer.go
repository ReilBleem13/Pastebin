package kafka

import (
	"cleanService/config"
	logging "cleanService/utils/logger"
	"fmt"

	"github.com/confluentinc/confluent-kafka-go/kafka"
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

func NewConsumer(handler Handler, logger *logging.Logger, cfg config.KafkaConfig, workers int) (*Consumer, error) {
	config := &kafka.ConfigMap{
		"bootstrap.servers":  cfg.Address,
		"group.id":           cfg.Group,
		"auto.offset.reset":  "latest",
		"enable.auto.commit": true,
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
		logger:   logger,
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
					c.logger.Errorf("handler error: %v", err)
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
				c.logger.Errorf("kafka read error: %v", err)
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
