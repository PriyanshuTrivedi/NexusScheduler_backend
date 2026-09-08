package mailer

import (
	"context"
	"fmt"
	"log"

	"github.com/PriyanshuTrivedi/nexus-scheduler/code/booking/entity"
	"github.com/PriyanshuTrivedi/nexus-scheduler/code/booking/mailer/risumailer"
	templaterenderer "github.com/PriyanshuTrivedi/nexus-scheduler/code/booking/mailer/template"
)

//go:generate mockgen -source=mailer.go -destination=../../../gen/mocks/booking/mailer/mailer_mock.go -package=mocks

type Mailer interface {
	Send(ctx context.Context, email entity.BookingEmail) error
}

type mailer struct {
	sender   risumailer.Sender
	renderer templaterenderer.Renderer
	from     string
}

func New(sender risumailer.Sender, renderer templaterenderer.Renderer, from string) Mailer {
	return &mailer{
		sender:   sender,
		renderer: renderer,
		from:     from,
	}
}

func (m *mailer) Send(ctx context.Context, email entity.BookingEmail) error {
	if !email.IsSendable() {
		return nil
	}

	if err := email.Validate(); err != nil {
		return fmt.Errorf("booking email validation: %w", err)
	}

	templateName := email.TemplateName()
	if templateName == "" {
		return fmt.Errorf("booking email: unsupported email type %q", email.Type)
	}

	body, err := m.renderer.Render(templateName, email)
	if err != nil {
		return err
	}

	err = m.sender.Send(ctx, m.from, email.To, email.Subject(), body)
	if err != nil {
		return fmt.Errorf("booking email: send to %s: %w", email.To, err)
	}

	log.Printf("booking email sent: type=%s to=%s reference=%s", email.Type, email.To, email.ReferenceCode)
	return nil
}
