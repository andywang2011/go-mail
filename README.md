# go-mail

SMTP mail helper for Go with a dev-friendly log mailer.

## Install

```bash
go get github.com/andywang2011/go-mail@v0.1.0
```

## Quick start

```go
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
```

## Examples

```bash
cd examples
go run .
```

## License

MIT — see [LICENSE](LICENSE).
