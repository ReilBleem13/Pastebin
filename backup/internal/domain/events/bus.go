package events

import (
	"context"
	"sync"
)

// EventHandler представляет обработчик событий
type EventHandler func(ctx context.Context, event Event) error

// EventBus представляет шину событий
type EventBus struct {
	handlers map[string][]EventHandler
	mu       sync.RWMutex
}

// NewEventBus создает новую шину событий
func NewEventBus() *EventBus {
	return &EventBus{
		handlers: make(map[string][]EventHandler),
	}
}

// Subscribe подписывает обработчик на определенный тип событий
func (b *EventBus) Subscribe(eventType string, handler EventHandler) {
	b.mu.Lock()
	defer b.mu.Unlock()

	if _, exists := b.handlers[eventType]; !exists {
		b.handlers[eventType] = make([]EventHandler, 0)
	}
	b.handlers[eventType] = append(b.handlers[eventType], handler)
}

// Publish публикует событие
func (b *EventBus) Publish(ctx context.Context, event Event) error {
	b.mu.RLock()
	handlers := b.handlers[event.GetEventType()]
	b.mu.RUnlock()

	var wg sync.WaitGroup
	errChan := make(chan error, len(handlers))

	for _, handler := range handlers {
		wg.Add(1)
		go func(h EventHandler) {
			defer wg.Done()
			if err := h(ctx, event); err != nil {
				errChan <- err
			}
		}(handler)
	}

	wg.Wait()
	close(errChan)

	// Проверяем ошибки
	for err := range errChan {
		if err != nil {
			return err
		}
	}

	return nil
}
