package stream

import (
	"crypto/rand"
	"fmt"
	"sync"

	datastar "github.com/starfederation/datastar-go/datastar"
)

// PushFn is a function that writes SSE events into an open generator.
type PushFn func(*datastar.ServerSentEventGenerator)

// Manager tracks active per-page SSE streams keyed by a random ID.
type Manager struct {
	mu      sync.Mutex
	clients map[string]chan PushFn
}

func NewManager() *Manager {
	return &Manager{clients: make(map[string]chan PushFn)}
}

// NewID returns a random hex stream ID.
func NewID() string {
	b := make([]byte, 16)
	rand.Read(b)
	return fmt.Sprintf("%x", b)
}

// Register creates a buffered channel for the given ID and returns a receive-only view.
func (m *Manager) Register(id string) <-chan PushFn {
	ch := make(chan PushFn, 32)
	m.mu.Lock()
	m.clients[id] = ch
	m.mu.Unlock()
	return ch
}

// Unregister removes and closes the channel for the given ID.
func (m *Manager) Unregister(id string) {
	m.mu.Lock()
	if ch, ok := m.clients[id]; ok {
		close(ch)
		delete(m.clients, id)
	}
	m.mu.Unlock()
}

// Push sends fn to the stream for id. Drops silently if the stream is full or absent.
func (m *Manager) Push(id string, fn PushFn) {
	m.mu.Lock()
	ch, ok := m.clients[id]
	m.mu.Unlock()
	if ok {
		select {
		case ch <- fn:
		default:
		}
	}
}
