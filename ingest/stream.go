package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"
)

type Hub struct {
	mu       sync.Mutex
	clients  map[chan []byte]struct{}
	lastPush map[string]time.Time
	counters *Counters
}

func NewHub(c *Counters) *Hub {
	return &Hub{
		clients:  make(map[chan []byte]struct{}),
		lastPush: make(map[string]time.Time),
		counters: c,
	}
}

func (h *Hub) subscribe() chan []byte {
	ch := make(chan []byte, 16)
	h.mu.Lock()
	h.clients[ch] = struct{}{}
	h.mu.Unlock()
	return ch
}

func (h *Hub) unsubscribe(ch chan []byte) {
	h.mu.Lock()
	delete(h.clients, ch)
	h.mu.Unlock()
}

// Publish is called from the MQTT handler. It never blocks.
func (h *Hub) Publish(id string, tr *DeviceTracker) {
	h.mu.Lock()
	now := time.Now()
	if now.Sub(h.lastPush[id]) < time.Second {
		h.mu.Unlock()
		return
	}
	h.lastPush[id] = now
	h.mu.Unlock()

	v, ok := tr.View(id)
	if !ok {
		return
	}
	data, err := json.Marshal(v)
	if err != nil {
		return
	}

	h.mu.Lock()
	defer h.mu.Unlock()
	for ch := range h.clients {
		select {
		case ch <- data:
		default:
			h.counters.Dropped.Add(1)
		}
	}
}

func (h *Hub) ServeStream(w http.ResponseWriter, r *http.Request) {
	fl, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")

	ch := h.subscribe()
	defer h.unsubscribe(ch)
	fl.Flush()

	for {
		select {
		case data := <-ch:
			fmt.Fprintf(w, "data: %s\n\n", data)
			fl.Flush()
		case <-r.Context().Done():
			return
		}
	}
}
