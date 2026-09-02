package smtp

import (
	"context"
	"fmt"

	"gopkg.in/gomail.v2"
)

type Sender interface {
	Send(ctx context.Context, from, to, subject, body string) error
}

type sender struct {
	dialer *gomail.Dialer
}

func New(host string, port int, username, password string) Sender {
	return &sender{
		dialer: gomail.NewDialer(host, port, username, password),
	}
}

func (s *sender) Send(ctx context.Context, from, to, subject, body string) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	message := gomail.NewMessage()
	message.SetHeader("From", from)
	message.SetHeader("To", to)
	message.SetHeader("Subject", subject)
	message.SetBody("text/html", body)

	if err := s.dialer.DialAndSend(message); err != nil {
		return fmt.Errorf("smtp send: %w", err)
	}

	return nil
}
