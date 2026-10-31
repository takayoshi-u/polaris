package mail_test

import (
	"strings"
	"testing"

	"github.com/takayoshi-u/polaris/backend/internal/mail"
)

func TestFormatMIMEAndCustomSend(t *testing.T) {
	mime := string(mail.FormatMIME("Polaris System", "noreply@example.com", "user@example.com", "Invite", "Hello World"))
	if !strings.Contains(mime, "From: Polaris System <noreply@example.com>") {
		t.Fatalf("expected From header in MIME, got:\n%s", mime)
	}
	if !strings.Contains(mime, "To: user@example.com") {
		t.Fatalf("expected To header in MIME, got:\n%s", mime)
	}
	if !strings.Contains(mime, "Subject: Invite") {
		t.Fatalf("expected Subject header in MIME, got:\n%s", mime)
	}

	sender := mail.NewSMTPSender()
	var capturedMsg mail.Message
	sender.CustomSend = func(cfg mail.Config, msg mail.Message) error {
		capturedMsg = msg
		return nil
	}
	if err := sender.Send(mail.Config{Enabled: true, Host: "smtp.example.com"}, mail.Message{
		To:      "alice@example.com",
		Subject: "Test",
		Body:    "Body",
	}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if capturedMsg.To != "alice@example.com" {
		t.Fatalf("expected captured recipient alice@example.com, got %s", capturedMsg.To)
	}
}

