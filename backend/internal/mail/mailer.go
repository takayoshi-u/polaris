package mail

import (
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"net/smtp"
	"strings"
	"time"
)

const (
	smtpDialTimeout = 3 * time.Second
	smtpIOTimeout   = 5 * time.Second
)

type Config struct {
	Enabled   bool
	Host      string
	Port      int
	UseAuth   bool
	Username  string
	Password  string
	FromEmail string
	FromName  string
}

type Message struct {
	To      string
	Subject string
	Body    string
}

type Sender interface {
	Send(cfg Config, msg Message) error
}

type SMTPSender struct {
	// CustomSend can be set in tests to intercept outgoing emails.
	CustomSend func(cfg Config, msg Message) error
}

func NewSMTPSender() *SMTPSender {
	return &SMTPSender{}
}

func FormatMIME(fromName, fromEmail, to, subject, body string) []byte {
	if fromEmail == "" {
		fromEmail = "noreply@localhost"
	}
	fromHeader := fromEmail
	if fromName != "" {
		fromHeader = fmt.Sprintf("%s <%s>", fromName, fromEmail)
	}
	headers := []string{
		"From: " + fromHeader,
		"To: " + to,
		"Subject: " + subject,
		"MIME-Version: 1.0",
		"Content-Type: text/plain; charset=\"UTF-8\"",
		"",
		body,
	}
	return []byte(strings.Join(headers, "\r\n"))
}

func (s *SMTPSender) Send(cfg Config, msg Message) error {
	if s.CustomSend != nil {
		return s.CustomSend(cfg, msg)
	}
	if msg.To == "" {
		return errors.New("recipient email is required")
	}
	fromEmail := cfg.FromEmail
	if fromEmail == "" {
		fromEmail = "noreply@localhost"
	}
	payload := FormatMIME(cfg.FromName, fromEmail, msg.To, msg.Subject, msg.Body)

	if cfg.Enabled {
		return sendViaExternalSMTP(cfg, fromEmail, msg.To, payload)
	}
	return sendDirectMX(fromEmail, msg.To, payload)
}

func sendViaExternalSMTP(cfg Config, fromEmail, to string, payload []byte) error {
	if cfg.Host == "" {
		return errors.New("external SMTP host is required when enabled")
	}
	port := cfg.Port
	if port <= 0 {
		port = 587
	}
	addr := fmt.Sprintf("%s:%d", cfg.Host, port)

	var auth smtp.Auth
	if cfg.UseAuth && cfg.Username != "" {
		auth = smtp.PlainAuth("", cfg.Username, cfg.Password, cfg.Host)
	}

	if port == 465 {
		tlsCfg := &tls.Config{ServerName: cfg.Host}
		conn, err := tls.DialWithDialer(&net.Dialer{Timeout: smtpDialTimeout}, "tcp", addr, tlsCfg)
		if err != nil {
			return err
		}
		_ = conn.SetDeadline(time.Now().Add(smtpIOTimeout))
		client, err := smtp.NewClient(conn, cfg.Host)
		if err != nil {
			_ = conn.Close()
			return err
		}
		defer client.Close()
		if auth != nil {
			if err := client.Auth(auth); err != nil {
				return err
			}
		}
		if err := client.Mail(fromEmail); err != nil {
			return err
		}
		if err := client.Rcpt(to); err != nil {
			return err
		}
		w, err := client.Data()
		if err != nil {
			return err
		}
		if _, err := w.Write(payload); err != nil {
			return err
		}
		if err := w.Close(); err != nil {
			return err
		}
		return client.Quit()
	}

	return sendSMTPWithTimeout(addr, cfg.Host, auth, fromEmail, to, payload)
}

func sendSMTPWithTimeout(addr, host string, auth smtp.Auth, fromEmail, to string, payload []byte) error {
	conn, err := net.DialTimeout("tcp", addr, smtpDialTimeout)
	if err != nil {
		return err
	}
	_ = conn.SetDeadline(time.Now().Add(smtpIOTimeout))
	client, err := smtp.NewClient(conn, host)
	if err != nil {
		_ = conn.Close()
		return err
	}
	defer client.Close()

	if auth != nil {
		if ok, _ := client.Extension("STARTTLS"); ok {
			tlsCfg := &tls.Config{ServerName: host}
			if err := client.StartTLS(tlsCfg); err != nil {
				return err
			}
		}
		if err := client.Auth(auth); err != nil {
			return err
		}
	}
	if err := client.Mail(fromEmail); err != nil {
		return err
	}
	if err := client.Rcpt(to); err != nil {
		return err
	}
	w, err := client.Data()
	if err != nil {
		return err
	}
	if _, err := w.Write(payload); err != nil {
		return err
	}
	if err := w.Close(); err != nil {
		return err
	}
	return client.Quit()
}

func sendDirectMX(fromEmail, to string, payload []byte) error {
	parts := strings.Split(to, "@")
	if len(parts) != 2 || parts[1] == "" {
		return errors.New("invalid recipient email address")
	}
	// If local mailpit dev server is available in Docker Compose, deliver immediately
	if err := sendSMTPWithTimeout("mailpit:1025", "mailpit", nil, fromEmail, to, payload); err == nil {
		return nil
	}

	domain := parts[1]
	mxRecords, err := net.LookupMX(domain)
	if err != nil || len(mxRecords) == 0 {
		return fmt.Errorf("failed to lookup MX records for domain %s: %w", domain, err)
	}

	var lastErr error
	for _, mx := range mxRecords {
		host := strings.TrimSuffix(mx.Host, ".")
		addr := net.JoinHostPort(host, "25")
		if err := sendSMTPWithTimeout(addr, host, nil, fromEmail, to, payload); err == nil {
			return nil
		} else {
			lastErr = err
		}
	}
	return fmt.Errorf("direct MX delivery failed: %w", lastErr)
}

