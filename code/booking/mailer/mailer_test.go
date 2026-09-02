package mailer

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/PriyanshuTrivedi/nexus-scheduler/code/booking/entity"
	"github.com/stretchr/testify/assert"
)

type fakeSender struct {
	from, to, subject, body string
	ctx                     context.Context
	err                     error
}

func (f *fakeSender) Send(ctx context.Context, from, to, subject, body string) error {
	f.ctx, f.from, f.to, f.subject, f.body = ctx, from, to, subject, body
	return f.err
}

type fakeRenderer struct {
	name string
	data any
	body string
	err  error
}

func (f *fakeRenderer) Render(name string, data any) (string, error) {
	f.name, f.data = name, data
	return f.body, f.err
}

func validBookingEmail() entity.BookingEmail {
	return entity.BookingEmail{
		Type: entity.EmailBookingOnline, To: "user@example.com", ReferenceCode: "NXS-ABC123",
		Start: time.Now().Add(time.Hour), End: time.Now().Add(2 * time.Hour), MeetingMode: "online",
	}
}

func TestNew(t *testing.T) {
	sender := &fakeSender{}
	renderer := &fakeRenderer{body: "body"}
	assert.NotNil(t, New(sender, renderer, "from@example.com"))
}

func TestMailerSend_Success(t *testing.T) {
	sender := &fakeSender{}
	renderer := &fakeRenderer{body: "<html>ok</html>"}
	m := New(sender, renderer, "nexus@example.com")
	e := validBookingEmail()
	err := m.Send(context.Background(), e)
	assert.NoError(t, err)
	assert.Equal(t, "booking_online.html", renderer.name)
	assert.Equal(t, "nexus@example.com", sender.from)
	assert.Equal(t, e.To, sender.to)
	assert.Equal(t, e.Subject(), sender.subject)
	assert.Equal(t, renderer.body, sender.body)
}

func TestMailerSend_SkipsBlankRecipient(t *testing.T) {
	sender := &fakeSender{}
	renderer := &fakeRenderer{}
	m := New(sender, renderer, "from@example.com")
	e := validBookingEmail()
	e.To = "  "
	assert.NoError(t, m.Send(context.Background(), e))
	assert.Empty(t, renderer.name)
	assert.Empty(t, sender.to)
}

func TestMailerSend_ValidationError(t *testing.T) {
	m := New(&fakeSender{}, &fakeRenderer{}, "from@example.com")
	e := validBookingEmail()
	e.ReferenceCode = ""
	err := m.Send(context.Background(), e)
	assert.EqualError(t, err, "booking email validation: email reference code is required")
}

func TestMailerSend_UnsupportedType(t *testing.T) {
	m := New(&fakeSender{}, &fakeRenderer{}, "from@example.com")
	e := validBookingEmail()
	e.Type = entity.EmailType("unsupported")
	err := m.Send(context.Background(), e)
	assert.Contains(t, err.Error(), `unsupported email type "unsupported"`)
}

func TestMailerSend_RenderError(t *testing.T) {
	renderErr := errors.New("render failed")
	renderer := &fakeRenderer{err: renderErr}
	m := New(&fakeSender{}, renderer, "from@example.com")
	err := m.Send(context.Background(), validBookingEmail())
	assert.ErrorIs(t, err, renderErr)
}

func TestMailerSend_SenderError(t *testing.T) {
	sendErr := errors.New("smtp failed")
	sender := &fakeSender{err: sendErr}
	m := New(sender, &fakeRenderer{body: "body"}, "from@example.com")
	err := m.Send(context.Background(), validBookingEmail())
	assert.ErrorIs(t, err, sendErr)
	assert.Contains(t, err.Error(), "booking email: send to user@example.com")
}
