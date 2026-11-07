package mail_test

import (
	"bytes"
	"context"
	"fmt"

	"github.com/andywang2011/go-mail"
)

func ExampleNewLogMailer() {
	buf := &bytes.Buffer{}
	dev := mail.NewLogMailer(buf)
	err := dev.Send(context.Background(), mail.Message{
		From:     "noreply@example.com",
		To:       []string{"user@example.com"},
		Subject:  "Welcome",
		TextBody: "Hello!",
	})
	if err != nil {
		panic(err)
	}
	fmt.Println(buf.Len() > 0)
	// Output: true
}
