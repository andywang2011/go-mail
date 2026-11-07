package mail

import "context"

// Mailer sends email messages (mock-friendly client surface).
type Mailer interface {
	// Send delivers msg. It returns an error when validation, transport, or context cancellation fails.
	Send(ctx context.Context, msg Message) error
}
