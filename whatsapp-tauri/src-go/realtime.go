package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"
)

const (
	realtimeClientBuffer = 32
	realtimeHeartbeat    = 20 * time.Second
)

type RealtimeEvent struct {
	Type       string   `json:"type"`
	ChatID     string   `json:"chatId,omitempty"`
	MessageIDs []string `json:"messageIds,omitempty"`
	Progress   *int     `json:"progress,omitempty"`
	SyncType   string   `json:"syncType,omitempty"`
}

type EventHub struct {
	mu          sync.RWMutex
	subscribers map[chan RealtimeEvent]struct{}
}

var realtimeHub = NewEventHub()

func NewEventHub() *EventHub {
	return &EventHub{subscribers: make(map[chan RealtimeEvent]struct{})}
}

func (h *EventHub) Subscribe() (<-chan RealtimeEvent, func()) {
	ch := make(chan RealtimeEvent, realtimeClientBuffer)
	h.mu.Lock()
	h.subscribers[ch] = struct{}{}
	h.mu.Unlock()

	var once sync.Once
	cancel := func() {
		once.Do(func() {
			h.mu.Lock()
			delete(h.subscribers, ch)
			close(ch)
			h.mu.Unlock()
		})
	}
	return ch, cancel
}

func (h *EventHub) Publish(event RealtimeEvent) {
	if event.Type == "" {
		return
	}
	h.mu.RLock()
	defer h.mu.RUnlock()
	for ch := range h.subscribers {
		select {
		case ch <- event:
		default:
			// Invalidations are coalescible. A slow consumer gets one full-resync
			// signal instead of blocking WhatsApp event processing or growing memory.
			for {
				select {
				case <-ch:
				default:
					ch <- RealtimeEvent{Type: "resync"}
					goto nextSubscriber
				}
			}
		}
	nextSubscriber:
	}
}

func (h *EventHub) SubscriberCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.subscribers)
}

func handleEvents(w http.ResponseWriter, r *http.Request) {
	handleEventsWithHub(w, r, realtimeHub)
}

func handleEventsWithHub(w http.ResponseWriter, r *http.Request, hub *EventHub) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w, http.MethodGet)
		return
	}
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming is not supported", http.StatusInternalServerError)
		return
	}
	if hub == nil {
		http.Error(w, "event stream is unavailable", http.StatusServiceUnavailable)
		return
	}

	updates, unsubscribe := hub.Subscribe()
	defer unsubscribe()
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache, no-transform")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)
	flusher.Flush()

	ticker := time.NewTicker(realtimeHeartbeat)
	defer ticker.Stop()
	for {
		select {
		case <-r.Context().Done():
			return
		case event, open := <-updates:
			if !open {
				return
			}
			payload, err := json.Marshal(event)
			if err != nil {
				log.Printf("[http] failed to encode realtime event %q: %v", event.Type, err)
				continue
			}
			if _, err := fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event.Type, payload); err != nil {
				return
			}
			flusher.Flush()
		case <-ticker.C:
			if _, err := fmt.Fprint(w, ": keep-alive\n\n"); err != nil {
				return
			}
			flusher.Flush()
		}
	}
}

func publishRealtime(event RealtimeEvent) {
	realtimeHub.Publish(event)
}
