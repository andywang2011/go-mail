package mail

import (
	"bytes"
	"context"
	"io"
	"testing"
)

func TestNewLogMailerNilWriter(t *testing.T) {
	m := NewLogMailer(nil)
	if err := m.Send(context.Background(), Message{
		From:     "dev@localhost",
		To:       []string{"a@b.c"},
		Subject:  "x",
		TextBody: "y",
	}); err != nil {
		t.Fatal(err)
	}
}

func TestLogMailerValidation(t *testing.T) {
	m := NewLogMailer(io.Discard)
	if err := m.Send(context.Background(), Message{}); err == nil {
		t.Fatal("expected validation error")
	}
}

func TestLogMailerContextDone(t *testing.T) {
	var buf bytes.Buffer
	m := NewLogMailer(&buf)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := m.Send(ctx, Message{
		To:       []string{"a@b.c"},
		Subject:  "x",
		TextBody: "y",
	}); err == nil {
		t.Fatal("expected context error")
	}
}
