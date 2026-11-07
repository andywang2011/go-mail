package main

import (
	"context"
	"os"

	"github.com/andywang2011/go-mail"
)

func main() {
	dev := mail.NewLogMailer(os.Stdout)
	_ = dev.Send(context.Background(), mail.Message{
		From:     "noreply@example.com",
		To:       []string{"user@example.com"},
		Subject:  "Welcome",
		TextBody: "Hello from go-mail examples!",
	})
}
