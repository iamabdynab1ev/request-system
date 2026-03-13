package websocket

import (
	"encoding/json"
	"log"
	"sync"
	"time"
	 "context"
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

func (h *Hub) Run(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			h.mu.Lock()
			for client := range h.clients {
				close(client.Send)
				delete(h.clients, client)
			}
			h.mu.Unlock()
			return
		case client := <-h.Register:
			h.mu.Lock()
			h.clients[client] = true
			h.userClients[client.UserID] = append(h.userClients[client.UserID], client)
			h.mu.Unlock()
		case client := <-h.unregister:
			h.mu.Lock()
			if _, ok := h.clients[client]; ok {
				delete(h.clients, client)
				close(client.Send)
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
			}
			h.mu.Unlock()
		case message := <-h.broadcast:
			h.mu.Lock()
			for client := range h.clients {
				select {
				case client.Send <- message:
				default:
					close(client.Send)
					delete(h.clients, client)
				}
			}
			h.mu.Unlock()
		}
	}
}
func (h *Hub) SendMessageToUser(userID uint64, payload interface{}, messageType string) error {
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

	h.mu.RLock()
	clients, ok := h.userClients[userID]
	if !ok {
		h.mu.RUnlock()
		log.Printf("Для userID %d не найдено активных соединений", userID)
		return nil
	}
	// Копируем срез чтобы отпустить мьютекс
	clientsCopy := make([]*Client, len(clients))
	copy(clientsCopy, clients)
	h.mu.RUnlock()

	log.Printf("Найдено %d активных соединений для userID %d", len(clientsCopy), userID)

	for _, client := range clientsCopy {
		select {
		case client.Send <- messageBytes:
		default:
			log.Printf("Канал клиента userID %d заполнен, пропускаем", userID)
		}
	}

	return nil
}
