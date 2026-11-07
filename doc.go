// Package mail provides a small, testable SMTP mail helper with config injection.
//
// Design constraints (for use in application frameworks):
//   - No viper, no global singletons, no log.Fatal inside the library.
//   - Constructors return Mailer (client_iface.go) for mocks and dependency injection.
//   - Use ConfigDefault() for SMTP tuning defaults; credentials still come from the caller.
//   - Send accepts context.Context for dial and IO deadlines.
//
// Use NewSMTP for production delivery and NewLogMailer for local development.
package mail
