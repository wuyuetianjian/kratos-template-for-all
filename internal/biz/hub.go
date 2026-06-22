package biz

import "sync"

// SessionHub tracks live WebSocket connections per session and delivers kick/expire signals.
type SessionHub struct {
	mu  sync.RWMutex
	chs map[int64]chan string
}

func newSessionHub() *SessionHub {
	return &SessionHub{chs: make(map[int64]chan string)}
}

// Register returns a channel that receives the event type when the session is terminated.
func (h *SessionHub) Register(sessionID int64) chan string {
	ch := make(chan string, 1)
	h.mu.Lock()
	h.chs[sessionID] = ch
	h.mu.Unlock()
	return ch
}

// Unregister removes the WebSocket entry for a session.
func (h *SessionHub) Unregister(sessionID int64) {
	h.mu.Lock()
	delete(h.chs, sessionID)
	h.mu.Unlock()
}

// Notify delivers a "kicked" signal to the WebSocket for the given session.
func (h *SessionHub) Notify(sessionID int64) {
	h.notify(sessionID, "kicked")
}

// NotifyExpired delivers a "expired" signal to the WebSocket for the given session.
func (h *SessionHub) NotifyExpired(sessionID int64) {
	h.notify(sessionID, "expired")
}

func (h *SessionHub) notify(sessionID int64, event string) {
	h.mu.RLock()
	ch, ok := h.chs[sessionID]
	h.mu.RUnlock()
	if ok {
		select {
		case ch <- event:
		default:
		}
	}
}
