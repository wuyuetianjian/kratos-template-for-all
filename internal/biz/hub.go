package biz

import "sync"

// SessionHub tracks live WebSocket connections per session and delivers kick signals.
type SessionHub struct {
	mu  sync.RWMutex
	chs map[int64]chan struct{}
}

func newSessionHub() *SessionHub {
	return &SessionHub{chs: make(map[int64]chan struct{})}
}

// Register returns a channel that receives a signal when the session is kicked.
func (h *SessionHub) Register(sessionID int64) chan struct{} {
	ch := make(chan struct{}, 1)
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

// Notify delivers a kick signal to the WebSocket connection for the given session.
func (h *SessionHub) Notify(sessionID int64) {
	h.mu.RLock()
	ch, ok := h.chs[sessionID]
	h.mu.RUnlock()
	if ok {
		select {
		case ch <- struct{}{}:
		default:
		}
	}
}
