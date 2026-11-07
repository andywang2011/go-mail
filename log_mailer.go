package mail

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
)

// logMailer writes messages as JSON lines for local development.
type logMailer struct {
	w io.Writer
}

// NewLogMailer returns a Mailer that logs each Message to w instead of sending SMTP traffic.
func NewLogMailer(w io.Writer) Mailer {
	if w == nil {
		w = io.Discard
	}
	return &logMailer{w: w}
}

// Send writes msg as a single JSON object followed by a newline.
func (m *logMailer) Send(ctx context.Context, msg Message) error {
	from := msg.From
	if err := validateMessage(msg, from); err != nil {
		return err
	}
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}
	b, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("mail: log encode: %w", err)
	}
	if _, err := fmt.Fprintf(m.w, "%s\n", b); err != nil {
		return fmt.Errorf("mail: log write: %w", err)
	}
	return nil
}
