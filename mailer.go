package mail

// Message is an outbound email payload.
type Message struct {
	// From is the RFC 5322 From address; when empty, Config.From is used.
	From string
	// To lists primary recipients.
	To []string
	// Subject is the message subject line.
	Subject string
	// HTMLBody is optional HTML content.
	HTMLBody string
	// TextBody is optional plain-text content.
	TextBody string
	// Headers holds extra MIME headers (e.g. List-Unsubscribe).
	Headers map[string]string
}
