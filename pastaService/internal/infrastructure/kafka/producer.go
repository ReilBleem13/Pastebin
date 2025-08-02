package kafka

import (
	"context"
	"encoding/json"
	"fmt"
	"pastebin/internal/config"
	"strings"

	"github.com/confluentinc/confluent-kafka-go/kafka"
	"github.com/theartofdevel/logging"
)

const (
	flushTimeout = 5000
)

type Producer struct {
	producer *kafka.Producer
}

func NewProducer(ctx context.Context, cfg config.KafkaConfig, transactionalID string) (*Producer, error) {
	logging.StringAttr("tranID", transactionalID)

	bootstrapServers := ""
	if cfg.Mode == "cluster" {
		bootstrapServers = strings.Join(cfg.Addrs, ", ")
	} else {
		bootstrapServers = cfg.Addr
	}

	conf := &kafka.ConfigMap{
		"bootstrap.servers":                     bootstrapServers,
		"acks":                                  cfg.Acks,
		"retries":                               cfg.Retries,
		"enable.idempotence":                    cfg.EnableIdempotence,
		"batch.num.messages":                    cfg.BatchNumMessages,
		"max.in.flight.requests.per.connection": cfg.MaxInFlightRequestsPerConn,
		"delivery.timeout.ms":                   cfg.DeliveryTimeoutMs,
		"request.timeout.ms":                    cfg.RequestTimeoutMs,
	}

	if transactionalID != "" {
		conf.SetKey("transactional.id", transactionalID)
	}

	p, err := kafka.NewProducer(conf)
	if err != nil {
		return nil, fmt.Errorf("failed to create new producer: %v", err)
	}

	if transactionalID != "" {
		if err := p.InitTransactions(ctx); err != nil {
			return nil, fmt.Errorf("failed to init transactions: %w", err)
		}
	}
	return &Producer{producer: p}, nil
}

func (p *Producer) Produce(ctx context.Context, event Event) error {
	value, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("failed to marshall event: %w", err)
	}

	msg := &kafka.Message{
		TopicPartition: kafka.TopicPartition{
			Topic:     &[]string{event.Topic()}[0],
			Partition: kafka.PartitionAny,
		},
		Value: value,
		Key:   []byte(event.Key()),
	}

	if err := p.producer.BeginTransaction(); err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}

	deliveryChan := make(chan kafka.Event, 1)
	if err := p.producer.Produce(msg, deliveryChan); err != nil {
		p.producer.AbortTransaction(ctx)
		return fmt.Errorf("produce failed: %w", err)
	}

	select {
	case ev := <-deliveryChan:
		m, ok := ev.(*kafka.Message)
		if !ok {
			p.producer.AbortTransaction(ctx)
			return fmt.Errorf("unexpected event type")
		}
		if m.TopicPartition.Error != nil {
			p.producer.AbortTransaction(ctx)
			return fmt.Errorf("delivary failed: %w", m.TopicPartition.Error)
		}
	case <-ctx.Done():
		p.producer.AbortTransaction(ctx)
		return ctx.Err()
	}
	close(deliveryChan)

	if err := p.producer.CommitTransaction(ctx); err != nil {
		p.producer.AbortTransaction(ctx)
		return fmt.Errorf("failed to commit transaction: %w", err)
	}
	return nil
}

func (p *Producer) Close() {
	p.producer.Flush(flushTimeout)
	p.producer.Close()
}
