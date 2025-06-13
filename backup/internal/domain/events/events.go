package events

import "time"

// Event представляет базовый интерфейс для всех событий
type Event interface {
	GetEventType() string
	GetTimestamp() time.Time
}

// PasteCreatedEvent событие создания новой пасты
type PasteCreatedEvent struct {
	PasteID    string
	UserID     int
	Visibility string
	CreatedAt  time.Time
}

func (e PasteCreatedEvent) GetEventType() string {
	return "paste.created"
}

func (e PasteCreatedEvent) GetTimestamp() time.Time {
	return e.CreatedAt
}

// UserRegisteredEvent событие регистрации нового пользователя
type UserRegisteredEvent struct {
	UserID    int
	Email     string
	CreatedAt time.Time
}

func (e UserRegisteredEvent) GetEventType() string {
	return "user.registered"
}

func (e UserRegisteredEvent) GetTimestamp() time.Time {
	return e.CreatedAt
}

// PasteDeletedEvent событие удаления пасты
type PasteDeletedEvent struct {
	PasteID   string
	UserID    int
	DeletedAt time.Time
}

func (e PasteDeletedEvent) GetEventType() string {
	return "paste.deleted"
}

func (e PasteDeletedEvent) GetTimestamp() time.Time {
	return e.DeletedAt
}
