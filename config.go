package mail

import (
	"errors"
	"fmt"
	"time"
)

const (
	defaultSMTPPort    = 587
	defaultMailTimeout = 30 * time.Second
)

// Config holds SMTP client settings injected by the application.
type Config struct {
	// Host is the SMTP server hostname (required).
	Host string
	// Port is the SMTP server port (default 587 when zero).
	Port int
	// Username is the SMTP AUTH username; optional when the server allows open relay.
	Username string
	// Password is the SMTP AUTH password; optional when Username is empty.
	Password string
	// From is the default RFC 5322 From address when Message.From is empty (required).
	From string
	// TLS enables TLS (implicit on port 465, STARTTLS otherwise when true).
	TLS bool
	// Timeout bounds dial and SMTP IO; default 30s when zero.
	Timeout time.Duration
}

// ConfigDefault returns Config with common SMTP defaults (port 587, TLS on, 30s timeout).
func ConfigDefault() Config {
	return Config{
		Port:    defaultSMTPPort,
		TLS:     true,
		Timeout: defaultMailTimeout,
	}
}

// Validate checks required fields and normalizes defaults.
func (c *Config) Validate() error {
	if c == nil {
		return errors.New("mail: config is nil")
	}
	if c.Host == "" {
		return errors.New("mail: host is not set")
	}
	if c.From == "" {
		return errors.New("mail: from is not set")
	}
	if c.Port == 0 {
		c.Port = defaultSMTPPort
	}
	if c.Timeout == 0 {
		c.Timeout = defaultMailTimeout
	}
	if c.Port < 1 || c.Port > 65535 {
		return fmt.Errorf("mail: invalid port %d", c.Port)
	}
	return nil
}
