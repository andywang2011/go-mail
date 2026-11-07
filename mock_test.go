package mail

import (
	"context"
	"sync"
)

// mockMailer records Send calls for tests. Copy this pattern in application tests.
type mockMailer struct {
	mu   sync.Mutex
	sent []Message
	err  error
}

func (m *mockMailer) Send(ctx context.Context, msg Message) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.err != nil {
		return m.err
	}
	m.sent = append(m.sent, msg)
	return nil
}

func (m *mockMailer) messages() []Message {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]Message, len(m.sent))
	copy(out, m.sent)
	return out
}
