package sse

import (
	"encoding/json"
	"fmt"
	"sync"
)

// Broker manages active client connections for SSE streaming.
type Broker struct {
	mu      sync.RWMutex
	clients map[string]map[chan []byte]struct{}
}

// NewBroker creates a new SSE Broker.
func NewBroker() *Broker {
	return &Broker{
		clients: make(map[string]map[chan []byte]struct{}),
	}
}

// Subscribe registers a listener for a specific run ID.
func (b *Broker) Subscribe(runID string) (chan []byte, func()) {
	b.mu.Lock()
	defer b.mu.Unlock()

	ch := make(chan []byte, 32)
	if _, exists := b.clients[runID]; !exists {
		b.clients[runID] = make(map[chan []byte]struct{})
	}
	b.clients[runID][ch] = struct{}{}

	cleanup := func() {
		b.mu.Lock()
		defer b.mu.Unlock()

		if subscribers, exists := b.clients[runID]; exists {
			delete(subscribers, ch)
			close(ch)
			if len(subscribers) == 0 {
				delete(b.clients, runID)
			}
		}
	}

	return ch, cleanup
}

// Broadcast sends a typed SSE message to all subscribers of a run ID.
func (b *Broker) Broadcast(runID string, eventType string, data any) {
	b.mu.RLock()
	defer b.mu.RUnlock()

	subscribers, exists := b.clients[runID]
	if !exists || len(subscribers) == 0 {
		return
	}

	payload, err := json.Marshal(data)
	if err != nil {
		return
	}

	msg := fmt.Sprintf("event: %s\ndata: %s\n\n", eventType, string(payload))
	msgBytes := []byte(msg)

	for ch := range subscribers {
		select {
		case ch <- msgBytes:
		default:
			// Buffer full, drop or let slow client catch up
		}
	}
}
