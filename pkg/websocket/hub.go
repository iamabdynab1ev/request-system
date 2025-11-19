package websocket

import (
	"encoding/json"
	"log"
	"sync"
	"time"
)

// Hub управляет всеми клиентами и рассылкой сообщений
type Hub struct {
	clients     map[*Client]bool
	userClients map[uint64][]*Client
	broadcast   chan []byte
	Register    chan *Client
	unregister  chan *Client
	mu          sync.RWMutex
}

func NewHub() *Hub {
	return &Hub{
		clients:     make(map[*Client]bool),
		userClients: make(map[uint64][]*Client),
		broadcast:   make(chan []byte),
		Register:    make(chan *Client),
		unregister:  make(chan *Client),
	}
}

func (h *Hub) Run() {
	for {
		select {
		case client := <-h.Register:
			h.mu.Lock()
			h.clients[client] = true
			h.userClients[client.UserID] = append(h.userClients[client.UserID], client)
			log.Printf("Клиент зарегистрирован: userID %d", client.UserID)
			h.mu.Unlock()
		case client := <-h.unregister:
			h.mu.Lock()
			if _, ok := h.clients[client]; ok {
				delete(h.clients, client)
				close(client.Send)
				// Удаляем клиента из среза userClients
				clients := h.userClients[client.UserID]
				for i, c := range clients {
					if c == client {
						h.userClients[client.UserID] = append(clients[:i], clients[i+1:]...)
						break
					}
				}
				if len(h.userClients[client.UserID]) == 0 {
					delete(h.userClients, client.UserID)
				}
				log.Printf("Клиент отсоединен: userID %d", client.UserID)
			}
			h.mu.Unlock()
		case message := <-h.broadcast:
			h.mu.RLock()
			for client := range h.clients {
				select {
				case client.Send <- message:
				default:
					close(client.Send)
					delete(h.clients, client)
				}
			}
			h.mu.RUnlock()
		}
	}
}

// SendMessageToUser — главный метод для отправки уведомления конкретному пользователю
func (h *Hub) SendMessageToUser(userID uint64, payload interface{}, messageType string) error {
	h.mu.RLock()
	defer h.mu.RUnlock()

	envelope := Envelope{
		Type:      messageType,
		Payload:   payload,
		Timestamp: time.Now().UTC(),
	}

	messageBytes, err := json.Marshal(envelope)
	if err != nil {
		log.Printf("Ошибка сериализации сообщения для WebSocket: %v", err)
		return err
	}

	if clients, ok := h.userClients[userID]; ok {
		log.Printf("Найдено %d активных соединений для userID %d", len(clients), userID)
		for _, client := range clients {
			client.Send <- messageBytes
		}
	} else {
		log.Printf("Для userID %d не найдено активных соединений", userID)
	}

	return nil
}
