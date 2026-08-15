package mailer

import (
	"context"
	"fmt"
	"log"
	"net/smtp"
	"time"
)

type Mailer interface {
	SendBookingConfirmation(ctx context.Context, to, name, referenceCode, resourceID, title string, start, end time.Time) error
}

type Config struct {
	Host     string
	Port     int
	Username string
	Password string
	From     string
}

type SMTPMailer struct{ cfg Config }

func NewSMTP(cfg Config) Mailer {
	if cfg.Host == "" || cfg.From == "" {
		return &ConsoleMailer{}
	}
	return &SMTPMailer{cfg: cfg}
}

func (m *SMTPMailer) SendBookingConfirmation(ctx context.Context, to, name, referenceCode, resourceID, title string, start, end time.Time) error {
	_ = ctx
	body := fmt.Sprintf("Hello %s,\n\nYour Nexus Scheduler booking is confirmed.\n\nReference: %s\nResource: %s\nTitle: %s\nStart: %s\nEnd: %s\n\nThank you.\n", name, referenceCode, resourceID, title, start.Format(time.RFC3339), end.Format(time.RFC3339))
	return m.send(to, "Nexus Scheduler booking confirmation", body)
}

func (m *SMTPMailer) send(to, subject, body string) error {
	addr := fmt.Sprintf("%s:%d", m.cfg.Host, m.cfg.Port)
	headers := "From: " + m.cfg.From + "\r\n" + "To: " + to + "\r\n" + "Subject: " + subject + "\r\n" + "MIME-Version: 1.0\r\n" + "Content-Type: text/plain; charset=UTF-8\r\n\r\n"
	var auth smtp.Auth
	if m.cfg.Username != "" {
		auth = smtp.PlainAuth("", m.cfg.Username, m.cfg.Password, m.cfg.Host)
	}
	return smtp.SendMail(addr, auth, m.cfg.From, []string{to}, []byte(headers+body))
}

type ConsoleMailer struct{}

func (m *ConsoleMailer) SendBookingConfirmation(ctx context.Context, to, name, referenceCode, resourceID, title string, start, end time.Time) error {
	_ = ctx
	log.Printf("booking confirmation email to=%s name=%s reference=%s resource=%s title=%s start=%s end=%s", to, name, referenceCode, resourceID, title, start.Format(time.RFC3339), end.Format(time.RFC3339))
	return nil
}
