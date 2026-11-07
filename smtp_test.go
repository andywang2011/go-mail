package mail

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"io"
	"net"
	"net/smtp"
	"strings"
	"testing"
	"time"
)

func validSMTPConfig() Config {
	return Config{
		Host: "127.0.0.1",
		From: "noreply@example.com",
		Port: 2525,
		TLS:  false,
	}
}

func TestNewSMTPValidateConfig(t *testing.T) {
	_, err := NewSMTP(Config{})
	if err == nil {
		t.Fatal("expected error for invalid config")
	}
}

func TestSendValidation(t *testing.T) {
	m, err := NewSMTPWithSendMail(validSMTPConfig(), func(context.Context, string, smtp.Auth, string, []string, []byte) error {
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	if err := m.Send(ctx, Message{}); err == nil {
		t.Fatal("expected validation error")
	}
	if err := m.Send(ctx, Message{
		To:      []string{"user@example.com"},
		Subject: "hi",
	}); err == nil {
		t.Fatal("expected body validation error")
	}
}

func TestSendWithInjectedSender(t *testing.T) {
	var gotFrom string
	var gotTo []string
	var gotRaw string
	m, err := NewSMTPWithSendMail(validSMTPConfig(), func(_ context.Context, _ string, _ smtp.Auth, from string, to []string, raw []byte) error {
		gotFrom = from
		gotTo = to
		gotRaw = string(raw)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	msg := Message{
		To:       []string{"alice@example.com"},
		Subject:  "Hello",
		TextBody: "plain",
		HTMLBody: "<p>html</p>",
		Headers:  map[string]string{"List-Unsubscribe": "<mailto:unsub@example.com>"},
	}
	if err := m.Send(context.Background(), msg); err != nil {
		t.Fatalf("Send: %v", err)
	}
	if gotFrom != validSMTPConfig().From {
		t.Fatalf("from: got %q", gotFrom)
	}
	if len(gotTo) != 1 || gotTo[0] != "alice@example.com" {
		t.Fatalf("to: %#v", gotTo)
	}
	if !strings.Contains(gotRaw, "multipart/alternative") {
		t.Fatalf("expected multipart body, got %q", gotRaw)
	}
	if !strings.Contains(gotRaw, "List-Unsubscribe") {
		t.Fatalf("expected custom header in raw message")
	}
}

func TestSendContextCanceled(t *testing.T) {
	m, err := NewSMTPWithSendMail(validSMTPConfig(), func(ctx context.Context, _ string, _ smtp.Auth, _ string, _ []string, _ []byte) error {
		return ctx.Err()
	})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err = m.Send(ctx, Message{
		To:       []string{"a@b.c"},
		Subject:  "x",
		TextBody: "y",
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got %v", err)
	}
}

func TestSendViaFakeSMTPServer(t *testing.T) {
	srv := newFakeSMTPServer(t)
	defer srv.Close()

	cfg := validSMTPConfig()
	cfg.Host = srv.host
	cfg.Port = srv.port
	cfg.Username = "user"
	cfg.Password = "pass"

	m, err := NewSMTP(cfg)
	if err != nil {
		t.Fatal(err)
	}
	err = m.Send(context.Background(), Message{
		To:       []string{"recipient@example.com"},
		Subject:  "Test",
		TextBody: "hello smtp",
	})
	if err != nil {
		t.Fatalf("Send: %v", err)
	}
	srv.waitDone(t)
	if srv.mailFrom != validSMTPConfig().From {
		t.Fatalf("mail from: got %q", srv.mailFrom)
	}
	if len(srv.rcptTo) != 1 || srv.rcptTo[0] != "recipient@example.com" {
		t.Fatalf("rcpt: %#v", srv.rcptTo)
	}
	if !strings.Contains(srv.body, "hello smtp") {
		t.Fatalf("body missing payload: %q", srv.body)
	}
	if !srv.authed {
		t.Fatal("expected SMTP AUTH")
	}
}

func TestSendTLSRequiredWithoutSTARTTLSFails(t *testing.T) {
	srv := newFakeSMTPServer(t) // advertises AUTH PLAIN but not STARTTLS
	defer srv.Close()

	cfg := validSMTPConfig()
	cfg.Host = srv.host
	cfg.Port = srv.port
	cfg.Username = "user"
	cfg.Password = "pass"
	cfg.TLS = true

	m, err := NewSMTP(cfg)
	if err != nil {
		t.Fatal(err)
	}
	err = m.Send(context.Background(), Message{
		To:       []string{"recipient@example.com"},
		Subject:  "Test",
		TextBody: "secret",
	})
	if !errors.Is(err, ErrSTARTTLSRequired) {
		t.Fatalf("err = %v, want ErrSTARTTLSRequired", err)
	}
	srv.waitDone(t)
	if srv.authed {
		t.Fatal("AUTH must not be sent in cleartext when STARTTLS is required")
	}
	if srv.mailFrom != "" || len(srv.rcptTo) != 0 || srv.body != "" {
		t.Fatalf("no mail transaction expected: from=%q rcpt=%v body=%q", srv.mailFrom, srv.rcptTo, srv.body)
	}
}

func TestSendSTARTTLSHandshakeFailureDoesNotAuth(t *testing.T) {
	srv := newFakeSMTPServerWith(t, true)
	defer srv.Close()

	cfg := validSMTPConfig()
	cfg.Host = srv.host
	cfg.Port = srv.port
	cfg.Username = "user"
	cfg.Password = "pass"
	cfg.TLS = true

	m, err := NewSMTP(cfg)
	if err != nil {
		t.Fatal(err)
	}
	err = m.Send(context.Background(), Message{
		To:       []string{"recipient@example.com"},
		Subject:  "Test",
		TextBody: "secret",
	})
	if err == nil || !strings.Contains(err.Error(), "starttls") {
		t.Fatalf("err = %v, want starttls error", err)
	}
	srv.waitDone(t)
	if srv.authed {
		t.Fatal("AUTH must not be sent after failed STARTTLS")
	}
}

func TestSendRejectsHeaderInjection(t *testing.T) {
	called := false
	m, err := NewSMTPWithSendMail(validSMTPConfig(), func(context.Context, string, smtp.Auth, string, []string, []byte) error {
		called = true
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	base := func() Message {
		return Message{To: []string{"a@example.com"}, Subject: "s", TextBody: "b"}
	}
	tests := []struct {
		name string
		mut  func(*Message)
	}{
		{"from CRLF", func(m *Message) { m.From = "a@example.com\r\nBcc: evil@example.com" }},
		{"from LF", func(m *Message) { m.From = "a@example.com\nBcc: evil@example.com" }},
		{"to CRLF", func(m *Message) { m.To = []string{"a@example.com\r\nBcc: evil@example.com"} }},
		{"second to CR", func(m *Message) { m.To = []string{"a@example.com", "b@example.com\rBcc: x"} }},
		{"subject CRLF", func(m *Message) { m.Subject = "hi\r\nBcc: evil@example.com" }},
		{"header value CRLF", func(m *Message) { m.Headers = map[string]string{"X-Foo": "bar\r\nBcc: evil@example.com"} }},
		{"header value LF", func(m *Message) { m.Headers = map[string]string{"X-Foo": "bar\nBcc: evil@example.com"} }},
		{"header name CRLF", func(m *Message) { m.Headers = map[string]string{"X-Foo\r\nBcc": "evil@example.com"} }},
		{"header name colon", func(m *Message) { m.Headers = map[string]string{"X-Foo: bar\r": "x"} }},
		{"header name colon only", func(m *Message) { m.Headers = map[string]string{"X:Foo": "x"} }},
		{"header name space", func(m *Message) { m.Headers = map[string]string{"X Foo": "x"} }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			called = false
			msg := base()
			tt.mut(&msg)
			err := m.Send(context.Background(), msg)
			if !errors.Is(err, ErrHeaderInjection) {
				t.Fatalf("err = %v, want ErrHeaderInjection", err)
			}
			if called {
				t.Fatal("sender must not be called for injected message")
			}
		})
	}
}

func TestSendAcceptsCleanHeaders(t *testing.T) {
	var raw string
	m, err := NewSMTPWithSendMail(validSMTPConfig(), func(_ context.Context, _ string, _ smtp.Auth, _ string, _ []string, r []byte) error {
		raw = string(r)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	err = m.Send(context.Background(), Message{
		From:     "Sender <s@example.com>",
		To:       []string{"a@example.com"},
		Subject:  "hello",
		TextBody: "b",
		Headers:  map[string]string{"X-Custom": "ok value"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(raw, "X-Custom: ok value\r\n") {
		t.Fatalf("raw = %q", raw)
	}
}

func TestLogMailer(t *testing.T) {
	var buf bytes.Buffer
	m := NewLogMailer(&buf)
	err := m.Send(context.Background(), Message{
		From:     "dev@localhost",
		To:       []string{"a@example.com"},
		Subject:  "dev",
		TextBody: "log me",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), `"Subject":"dev"`) {
		t.Fatalf("unexpected log: %s", buf.String())
	}
}

func TestMockMailer(t *testing.T) {
	mock := &mockMailer{}
	ctx := context.Background()
	msg := Message{To: []string{"x@y.z"}, Subject: "s", TextBody: "b"}
	if err := mock.Send(ctx, msg); err != nil {
		t.Fatal(err)
	}
	sent := mock.messages()
	if len(sent) != 1 || sent[0].Subject != "s" {
		t.Fatalf("sent: %#v", sent)
	}
}

type fakeSMTPServer struct {
	ln       net.Listener
	host     string
	port     int
	done     chan struct{}
	mailFrom string
	rcptTo   []string
	body     string
	authed   bool
	// offerStartTLS makes the server advertise STARTTLS and then abort the handshake.
	offerStartTLS bool
}

func newFakeSMTPServer(t *testing.T) *fakeSMTPServer {
	t.Helper()
	return newFakeSMTPServerWith(t, false)
}

// newFakeSMTPServerWith starts a fake server; when offerStartTLS is true it advertises
// STARTTLS and drops the connection after the 220 reply so the client handshake fails.
func newFakeSMTPServerWith(t *testing.T, offerStartTLS bool) *fakeSMTPServer {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := ln.Addr().(*net.TCPAddr)
	s := &fakeSMTPServer{
		ln:   ln,
		host: addr.IP.String(),
		port: addr.Port,
		done: make(chan struct{}),

		offerStartTLS: offerStartTLS,
	}
	go s.serve()
	return s
}

func (s *fakeSMTPServer) Close() {
	_ = s.ln.Close()
	select {
	case <-s.done:
	case <-time.After(2 * time.Second):
	}
}

func (s *fakeSMTPServer) waitDone(t *testing.T) {
	t.Helper()
	select {
	case <-s.done:
	case <-time.After(3 * time.Second):
		t.Fatal("timeout waiting for fake SMTP session")
	}
}

func (s *fakeSMTPServer) serve() {
	defer close(s.done)
	conn, err := s.ln.Accept()
	if err != nil {
		return
	}
	defer conn.Close()
	rw := bufio.NewReadWriter(bufio.NewReader(conn), bufio.NewWriter(conn))
	writeLine := func(line string) {
		_, _ = io.WriteString(rw, line+"\r\n")
		_ = rw.Flush()
	}
	readLine := func() string {
		line, err := rw.ReadString('\n')
		if err != nil {
			return ""
		}
		return strings.TrimSpace(line)
	}

	writeLine("220 fake.local ESMTP")
	for {
		line := readLine()
		if line == "" {
			return
		}
		upper := strings.ToUpper(line)
		switch {
		case strings.HasPrefix(upper, "EHLO") || strings.HasPrefix(upper, "HELO"):
			writeLine("250-fake.local")
			if s.offerStartTLS {
				writeLine("250-STARTTLS")
			}
			writeLine("250-AUTH PLAIN")
			writeLine("250 OK")
		case strings.HasPrefix(upper, "STARTTLS"):
			if s.offerStartTLS {
				writeLine("220 Ready to start TLS")
				return
			}
			writeLine("502 STARTTLS not supported")
		case strings.HasPrefix(upper, "AUTH"):
			s.authed = true
			writeLine("235 Authentication successful")
		case strings.HasPrefix(upper, "MAIL FROM"):
			s.mailFrom = strings.TrimSuffix(strings.TrimPrefix(line, "MAIL FROM:"), ">")
			s.mailFrom = strings.Trim(strings.TrimPrefix(s.mailFrom, "<"), ">")
			writeLine("250 OK")
		case strings.HasPrefix(upper, "RCPT TO"):
			rcpt := strings.TrimSuffix(strings.TrimPrefix(line, "RCPT TO:"), ">")
			rcpt = strings.Trim(strings.TrimPrefix(rcpt, "<"), ">")
			s.rcptTo = append(s.rcptTo, rcpt)
			writeLine("250 OK")
		case upper == "DATA":
			writeLine("354 End data with <CR><LF>.<CR><LF>")
			var body strings.Builder
			for {
				part, err := rw.ReadString('\n')
				if err != nil {
					return
				}
				if part == ".\r\n" || part == ".\n" {
					break
				}
				body.WriteString(part)
			}
			s.body = body.String()
			writeLine("250 OK")
		case strings.HasPrefix(upper, "QUIT"):
			writeLine("221 Bye")
			return
		default:
			writeLine("250 OK")
		}
	}
}

var _ Mailer = (*mockMailer)(nil)
