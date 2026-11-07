# Changelog

## [Unreleased]

## [0.2.0] - 2026-11-07

### Breaking

- See repository diff for API changes in this release (secure defaults, renamed packages where applicable, context-aware constructors).

### Security

- Hardened defaults: TLS, timeouts, body limits, PII redaction where logging applies.

### Fixed

- Bug fixes from the v0.2.0 hardening pass.

### Changed

- CI uses go-version-file; added race, staticcheck, govulncheck, golangci-lint (gosec).



- Security: when TLS is required and the server does not offer STARTTLS, sending fails (`ErrSTARTTLSRequired`) before AUTH.
- Security: CR/LF in addresses, subject, and custom headers (and invalid header names) are rejected (`ErrHeaderInjection`).

## [0.1.0]

- Initial release: SMTP `Mailer` with config injection and injectable send for tests.
- `NewLogMailer` for local JSON-line logging without network.
