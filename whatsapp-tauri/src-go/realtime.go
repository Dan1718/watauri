package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"
)

const (
	realtimeClientBuffer     = 32
	realtimeHeartbeat        = 20 * time.Second
	historySyncFlushInterval = time.Minute
)

type RealtimeEvent struct {
	Type         string   `json:"type"`
	ChatID       string   `json:"chatId,omitempty"`
	MessageIDs   []string `json:"messageIds,omitempty"`
	ClientSendID string   `json:"clientSendId,omitempty"`
	Progress     *int     `json:"progress,omitempty"`
	SyncType     string   `json:"syncType,omitempty"`
}

type EventHub struct {
	mu           sync.RWMutex
	subscribers  map[chan RealtimeEvent]struct{}
	syncTypes    map[string]struct{}
	syncProgress map[string]int
	pending      bool
	flushTimer   *time.Timer
}

var realtimeHub = NewEventHub()

func NewEventHub() *EventHub {
	return &EventHub{
		subscribers:  make(map[chan RealtimeEvent]struct{}),
		syncTypes:    make(map[string]struct{}),
		syncProgress: make(map[string]int),
	}
}

// BeginHistorySync coalesces invalidations while one or more history syncs run.
func (h *EventHub) BeginHistorySync(syncType string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if _, exists := h.syncTypes[syncType]; exists {
		return
	}
	wasActive := len(h.syncTypes) > 0
	h.syncTypes[syncType] = struct{}{}
	h.syncProgress[syncType] = 0
	zero := 0
	h.publishLocked(RealtimeEvent{Type: "sync", Progress: &zero, SyncType: syncType})
	if !wasActive {
		h.flushTimer = time.AfterFunc(historySyncFlushInterval, h.flushDuringHistorySync)
	}
}

// EndHistorySync flushes pending invalidations immediately after the last sync ends.
func (h *EventHub) EndHistorySync(syncType string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	delete(h.syncTypes, syncType)
	if len(h.syncTypes) > 0 {
		return
	}
	if h.flushTimer != nil {
		h.flushTimer.Stop()
		h.flushTimer = nil
	}
	if h.pending {
		h.pending = false
		h.publishLocked(RealtimeEvent{Type: "resync"})
	}
}

func (h *EventHub) flushDuringHistorySync() {
	h.mu.Lock()
	defer h.mu.Unlock()
	if len(h.syncTypes) == 0 {
		h.flushTimer = nil
		return
	}
	if h.pending {
		h.pending = false
		h.publishLocked(RealtimeEvent{Type: "resync"})
	}
	h.flushTimer = time.AfterFunc(historySyncFlushInterval, h.flushDuringHistorySync)
}

func (h *EventHub) Subscribe() (<-chan RealtimeEvent, func()) {
	ch := make(chan RealtimeEvent, realtimeClientBuffer)
	h.mu.Lock()
	h.subscribers[ch] = struct{}{}
	for syncType, progress := range h.syncProgress {
		progress := progress
		ch <- RealtimeEvent{Type: "sync", Progress: &progress, SyncType: syncType}
	}
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
	started := time.Now()
	h.mu.Lock()
	if event.Type == "sync" && event.SyncType != "" && event.Progress != nil {
		if *event.Progress >= 100 {
			delete(h.syncProgress, event.SyncType)
		} else {
			h.syncProgress[event.SyncType] = *event.Progress
		}
	}
	if len(h.syncTypes) > 0 && isInvalidationEvent(event.Type) {
		h.pending = true
		h.mu.Unlock()
		traceEvent(context.Background(), "realtime.publish", map[string]any{"type": event.Type, "coalesced": true, "duration_ns": time.Since(started).Nanoseconds()})
		return
	}
	h.publishLocked(event)
	h.mu.Unlock()
}

func isInvalidationEvent(eventType string) bool {
	switch eventType {
	case "message", "receipt", "chat", "contact", "resync":
		return true
	default:
		return false
	}
}

func (h *EventHub) publishLocked(event RealtimeEvent) {
	started := time.Now()
	subscriberCount := len(h.subscribers)
	replaced := 0
	for ch := range h.subscribers {
		select {
		case ch <- event:
		default:
			replaced++
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
	traceEvent(context.Background(), "realtime.publish", map[string]any{"type": event.Type, "subscribers": subscriberCount, "slow_subscriber_resyncs": replaced, "duration_ns": time.Since(started).Nanoseconds()})
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
			writeStarted := time.Now()
			payload, err := json.Marshal(event)
			if err != nil {
				traceEvent(r.Context(), "realtime.encode", map[string]any{"type": event.Type, "duration_ns": time.Since(writeStarted).Nanoseconds(), "failed": true, "error_type": errorType(err)})
				log.Printf("[http] failed to encode realtime event %q: %v", event.Type, err)
				continue
			}
			traceEvent(r.Context(), "realtime.encode", map[string]any{"type": event.Type, "duration_ns": time.Since(writeStarted).Nanoseconds(), "bytes": len(payload)})
			writeStarted = time.Now()
			written, err := fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event.Type, payload)
			if err != nil {
				traceEvent(r.Context(), "realtime.write", map[string]any{"type": event.Type, "duration_ns": time.Since(writeStarted).Nanoseconds(), "failed": true, "error_type": errorType(err)})
				return
			}
			flusher.Flush()
			traceEvent(r.Context(), "realtime.write", map[string]any{"type": event.Type, "bytes": written, "duration_ns": time.Since(writeStarted).Nanoseconds()})
		case <-ticker.C:
			writeStarted := time.Now()
			if _, err := fmt.Fprint(w, ": keep-alive\n\n"); err != nil {
				traceEvent(r.Context(), "realtime.heartbeat", map[string]any{"duration_ns": time.Since(writeStarted).Nanoseconds(), "failed": true, "error_type": errorType(err)})
				return
			}
			flusher.Flush()
			traceEvent(r.Context(), "realtime.heartbeat", map[string]any{"duration_ns": time.Since(writeStarted).Nanoseconds()})
		}
	}
}

func publishRealtime(event RealtimeEvent) {
	realtimeHub.Publish(event)
}
