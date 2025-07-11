package kafka

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/confluentinc/confluent-kafka-go/kafka"
)

const (
	flushTimeout = 5000
)

type Producer struct {
	producer *kafka.Producer
}

func NewProducer(address string) (*Producer, error) {
	fmt.Println("Kafka address:", address)
	conf := &kafka.ConfigMap{
		"bootstrap.servers": address,
		"acks":              "1",
		"retries":           3,
	}
	p, err := kafka.NewProducer(conf)
	if err != nil {
		return nil, fmt.Errorf("failed to create new producer: %v", err)
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
	kafkaChan := make(chan kafka.Event, 1)
	if err := p.producer.Produce(msg, kafkaChan); err != nil {
		return fmt.Errorf("produce failed: %w", err)
	}
	select {
	case ev := <-kafkaChan:
		m, ok := ev.(*kafka.Message)
		if !ok {
			return fmt.Errorf("unexpected event type")
		}
		if m.TopicPartition.Error != nil {
			return fmt.Errorf("delivary failed: %w", m.TopicPartition.Error)
		}
	case <-ctx.Done():
		return ctx.Err()
	}
	return nil
}

func (p *Producer) Close() {
	p.producer.Flush(flushTimeout)
	p.producer.Close()
}
