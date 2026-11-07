package mail

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/base64"
	"errors"
	"fmt"
	"mime"
	"mime/quotedprintable"
	"net"
	"net/smtp"
	"strconv"
	"strings"
	"time"
)

var (
	// ErrSTARTTLSRequired is returned when TLS is required but the server does not offer
	// STARTTLS. Credentials are never sent over a cleartext connection in that case.
	ErrSTARTTLSRequired = errors.New("mail: STARTTLS required but not offered by server")
	// ErrHeaderInjection is returned when an address, subject, or header contains CR or LF.
	ErrHeaderInjection = errors.New("mail: header injection detected")
)

// sendMailFunc sends a raw RFC 822 message; injected in tests via NewSMTPWithSendMail.
type sendMailFunc func(ctx context.Context, addr string, auth smtp.Auth, from string, to []string, raw []byte) error

type smtpMailer struct {
	cfg  Config
	send sendMailFunc
}

// NewSMTP creates an SMTP Mailer from cfg.
func NewSMTP(cfg Config) (Mailer, error) {
	return NewSMTPWithSendMail(cfg, nil)
}

// NewSMTPWithSendMail creates an SMTP Mailer and optionally overrides the send implementation (for tests).
func NewSMTPWithSendMail(cfg Config, send sendMailFunc) (Mailer, error) {
	c := cfg
	if err := c.Validate(); err != nil {
		return nil, err
	}
	m := &smtpMailer{cfg: c}
	if send != nil {
		m.send = send
	} else {
		m.send = m.deliver
	}
	return m, nil
}

// Send delivers msg over SMTP.
func (m *smtpMailer) Send(ctx context.Context, msg Message) error {
	from := msg.From
	if from == "" {
		from = m.cfg.From
	}
	if err := validateMessage(msg, from); err != nil {
		return err
	}
	raw, err := buildRawMessage(from, msg)
	if err != nil {
		return err
	}
	addr := net.JoinHostPort(m.cfg.Host, strconv.Itoa(m.cfg.Port))
	var auth smtp.Auth
	if m.cfg.Username != "" {
		auth = smtp.PlainAuth("", m.cfg.Username, m.cfg.Password, m.cfg.Host)
	}
	return m.send(ctx, addr, auth, from, msg.To, raw)
}

func (m *smtpMailer) deliver(ctx context.Context, addr string, auth smtp.Auth, from string, to []string, raw []byte) error {
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		return fmt.Errorf("mail: invalid smtp addr: %w", err)
	}
	dialer := &net.Dialer{Timeout: timeoutFromContext(ctx, m.cfg.Timeout)}
	conn, err := dialer.DialContext(ctx, "tcp", addr)
	if err != nil {
		return fmt.Errorf("mail: smtp dial: %w", err)
	}
	defer conn.Close()

	var client *smtp.Client
	portStr := addr[strings.LastIndex(addr, ":")+1:]
	port, _ := strconv.Atoi(portStr)
	useImplicitTLS := m.cfg.TLS && port == 465
	if useImplicitTLS {
		tlsConn := tls.Client(conn, &tls.Config{ServerName: host})
		if err := tlsConn.HandshakeContext(ctx); err != nil {
			return fmt.Errorf("mail: tls handshake: %w", err)
		}
		client, err = smtp.NewClient(tlsConn, host)
		if err != nil {
			return fmt.Errorf("mail: smtp client: %w", err)
		}
	} else {
		client, err = smtp.NewClient(conn, host)
		if err != nil {
			return fmt.Errorf("mail: smtp client: %w", err)
		}
	}

	defer func() { _ = client.Close() }()

	if m.cfg.TLS && !useImplicitTLS {
		if ok, _ := client.Extension("STARTTLS"); !ok {
			return ErrSTARTTLSRequired
		}
		if err := client.StartTLS(&tls.Config{ServerName: host}); err != nil {
			return fmt.Errorf("mail: starttls: %w", err)
		}
	}

	if auth != nil {
		if ok, _ := client.Extension("AUTH"); ok {
			if err := client.Auth(auth); err != nil {
				return fmt.Errorf("mail: smtp auth: %w", err)
			}
		}
	}

	if err := client.Mail(from); err != nil {
		return fmt.Errorf("mail: mail from: %w", err)
	}
	for _, rcpt := range to {
		if err := client.Rcpt(rcpt); err != nil {
			return fmt.Errorf("mail: rcpt to: %w", err)
		}
	}
	w, err := client.Data()
	if err != nil {
		return fmt.Errorf("mail: data: %w", err)
	}
	if _, err := w.Write(raw); err != nil {
		return fmt.Errorf("mail: write body: %w", err)
	}
	if err := w.Close(); err != nil {
		return fmt.Errorf("mail: close data: %w", err)
	}
	return client.Quit()
}

func validateMessage(msg Message, from string) error {
	if from == "" {
		return errors.New("mail: from is not set")
	}
	if err := rejectHeaderInjection("from", from); err != nil {
		return err
	}
	if len(msg.To) == 0 {
		return errors.New("mail: at least one recipient is required")
	}
	for i, to := range msg.To {
		if strings.TrimSpace(to) == "" {
			return fmt.Errorf("mail: recipient at index %d is empty", i)
		}
		if err := rejectHeaderInjection(fmt.Sprintf("to[%d]", i), to); err != nil {
			return err
		}
	}
	if msg.Subject == "" {
		return errors.New("mail: subject is required")
	}
	if err := rejectHeaderInjection("subject", msg.Subject); err != nil {
		return err
	}
	if msg.TextBody == "" && msg.HTMLBody == "" {
		return errors.New("mail: text or html body is required")
	}
	for k, v := range msg.Headers {
		if err := rejectHeaderName(k); err != nil {
			return err
		}
		if err := rejectHeaderInjection("header "+k, v); err != nil {
			return err
		}
	}
	return nil
}

func rejectHeaderInjection(field, value string) error {
	if strings.ContainsAny(value, "\r\n") {
		return fmt.Errorf("%w: %s contains CR/LF", ErrHeaderInjection, field)
	}
	return nil
}

// rejectHeaderName rejects empty or malformed custom header names in addition to CR/LF.
func rejectHeaderName(name string) error {
	if err := rejectHeaderInjection("header name", name); err != nil {
		return err
	}
	if strings.ContainsAny(name, ": \t") {
		return fmt.Errorf("%w: header name %q contains invalid characters", ErrHeaderInjection, name)
	}
	return nil
}

func buildRawMessage(from string, msg Message) ([]byte, error) {
	var buf bytes.Buffer
	writeHeader := func(k, v string) {
		fmt.Fprintf(&buf, "%s: %s\r\n", k, v)
	}

	writeHeader("From", from)
	writeHeader("To", strings.Join(msg.To, ", "))
	writeHeader("Subject", mime.QEncoding.Encode("utf-8", msg.Subject))
	writeHeader("MIME-Version", "1.0")
	writeHeader("Date", time.Now().UTC().Format(time.RFC1123Z))

	for k, v := range msg.Headers {
		if k == "" {
			continue
		}
		writeHeader(k, v)
	}

	hasText := msg.TextBody != ""
	hasHTML := msg.HTMLBody != ""

	switch {
	case hasText && hasHTML:
		boundary := randomBoundary()
		writeHeader("Content-Type", fmt.Sprintf(`multipart/alternative; boundary="%s"`, boundary))
		buf.WriteString("\r\n")
		if err := writeQuotedPart(&buf, boundary, "text/plain; charset=UTF-8", msg.TextBody); err != nil {
			return nil, err
		}
		if err := writeQuotedPart(&buf, boundary, "text/html; charset=UTF-8", msg.HTMLBody); err != nil {
			return nil, err
		}
		fmt.Fprintf(&buf, "--%s--\r\n", boundary)
	case hasHTML:
		writeHeader("Content-Type", "text/html; charset=UTF-8")
		writeHeader("Content-Transfer-Encoding", "quoted-printable")
		buf.WriteString("\r\n")
		qw := quotedprintable.NewWriter(&buf)
		if _, err := qw.Write([]byte(msg.HTMLBody)); err != nil {
			return nil, fmt.Errorf("mail: encode html: %w", err)
		}
		if err := qw.Close(); err != nil {
			return nil, fmt.Errorf("mail: close html encoder: %w", err)
		}
		buf.WriteString("\r\n")
	default:
		writeHeader("Content-Type", "text/plain; charset=UTF-8")
		writeHeader("Content-Transfer-Encoding", "quoted-printable")
		buf.WriteString("\r\n")
		qw := quotedprintable.NewWriter(&buf)
		if _, err := qw.Write([]byte(msg.TextBody)); err != nil {
			return nil, fmt.Errorf("mail: encode text: %w", err)
		}
		if err := qw.Close(); err != nil {
			return nil, fmt.Errorf("mail: close text encoder: %w", err)
		}
		buf.WriteString("\r\n")
	}
	return buf.Bytes(), nil
}

func writeQuotedPart(buf *bytes.Buffer, boundary, contentType, body string) error {
	fmt.Fprintf(buf, "--%s\r\n", boundary)
	fmt.Fprintf(buf, "Content-Type: %s\r\n", contentType)
	buf.WriteString("Content-Transfer-Encoding: quoted-printable\r\n\r\n")
	qw := quotedprintable.NewWriter(buf)
	if _, err := qw.Write([]byte(body)); err != nil {
		return fmt.Errorf("mail: encode part: %w", err)
	}
	if err := qw.Close(); err != nil {
		return fmt.Errorf("mail: close part encoder: %w", err)
	}
	buf.WriteString("\r\n")
	return nil
}

func randomBoundary() string {
	return "go-mail-" + base64.RawURLEncoding.EncodeToString([]byte(strconv.FormatInt(time.Now().UnixNano(), 36)))
}

func timeoutFromContext(ctx context.Context, fallback time.Duration) time.Duration {
	if deadline, ok := ctx.Deadline(); ok {
		d := time.Until(deadline)
		if d > 0 {
			return d
		}
	}
	if fallback > 0 {
		return fallback
	}
	return defaultMailTimeout
}
