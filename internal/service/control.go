package service

import (
	"sync"
	"time"

	"jdShopServer/internal/model"
)

// ControlHub distributes account control notifications to the active clients
// of a specific user. Notifications are deliberately transient: the client
// always confirms the current state through the heartbeat endpoint.
type ControlHub struct {
	mu          sync.RWMutex
	nextID      uint64
	subscribers map[int64]map[uint64]chan model.ControlEvent
}

func NewControlHub() *ControlHub {
	return &ControlHub{subscribers: make(map[int64]map[uint64]chan model.ControlEvent)}
}

func (h *ControlHub) Subscribe(userID int64) (<-chan model.ControlEvent, func()) {
	h.mu.Lock()
	h.nextID++
	subscriberID := h.nextID
	channel := make(chan model.ControlEvent, 1)
	if h.subscribers[userID] == nil {
		h.subscribers[userID] = make(map[uint64]chan model.ControlEvent)
	}
	h.subscribers[userID][subscriberID] = channel
	h.mu.Unlock()

	var once sync.Once
	unsubscribe := func() {
		once.Do(func() {
			h.mu.Lock()
			defer h.mu.Unlock()
			delete(h.subscribers[userID], subscriberID)
			if len(h.subscribers[userID]) == 0 {
				delete(h.subscribers, userID)
			}
			close(channel)
		})
	}
	return channel, unsubscribe
}

func (h *ControlHub) Publish(userID int64, eventType string) {
	event := model.ControlEvent{
		Type:     eventType,
		IssuedAt: time.Now().UTC().Format(time.RFC3339),
	}

	h.mu.RLock()
	defer h.mu.RUnlock()
	for _, channel := range h.subscribers[userID] {
		select {
		case channel <- event:
		default:
			// A notification only tells the client to fetch the authoritative
			// access state. Keeping one pending signal is sufficient.
		}
	}
}
