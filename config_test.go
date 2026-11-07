package mail

import (
	"testing"
	"time"
)

func TestConfigDefault(t *testing.T) {
	cfg := ConfigDefault()
	if cfg.Port != defaultSMTPPort {
		t.Fatalf("port: got %d want %d", cfg.Port, defaultSMTPPort)
	}
	if !cfg.TLS {
		t.Fatal("expected TLS enabled by default")
	}
	if cfg.Timeout != defaultMailTimeout {
		t.Fatalf("timeout: got %v want %v", cfg.Timeout, defaultMailTimeout)
	}
}

func TestConfigValidate(t *testing.T) {
	cfg := ConfigDefault()
	cfg.Host = "smtp.example.com"
	cfg.From = "noreply@example.com"
	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate: %v", err)
	}
	if cfg.Port != defaultSMTPPort {
		t.Fatalf("Validate should set port: got %d", cfg.Port)
	}

	bad := Config{}
	if err := bad.Validate(); err == nil {
		t.Fatal("expected error for empty config")
	}

	invalidPort := Config{Host: "h", From: "a@b.c", Port: 70000}
	if err := invalidPort.Validate(); err == nil {
		t.Fatal("expected error for invalid port")
	}
}

func TestConfigValidateNil(t *testing.T) {
	var cfg *Config
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected error for nil config")
	}
}

func TestConfigValidateCustomTimeout(t *testing.T) {
	cfg := Config{
		Host:    "smtp.example.com",
		From:    "noreply@example.com",
		Port:    2525,
		Timeout: 5 * time.Second,
	}
	if err := cfg.Validate(); err != nil {
		t.Fatal(err)
	}
	if cfg.Timeout != 5*time.Second {
		t.Fatalf("timeout mutated: %v", cfg.Timeout)
	}
}
