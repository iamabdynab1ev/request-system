package events

import (
	"request-system/internal/repositories"
)

// OrderHistoryCreatedEvent - событие, которое возникает после создания новой записи в истории.
type OrderHistoryCreatedEvent struct {
	HistoryItem repositories.OrderHistoryItem
	Order       interface{} // Мы можем передать сюда полную сущность Order
	Actor       interface{} // ... и сущность User, который совершил действие
}

// Name - реализуем интерфейс eventbus.Event
func (e OrderHistoryCreatedEvent) Name() string {
	return "order.history.created"
}
