package mailer

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/PriyanshuTrivedi/nexus-scheduler/code/booking/entity"
	templaterenderer "github.com/PriyanshuTrivedi/nexus-scheduler/code/booking/mailer/template"
	mocks "github.com/PriyanshuTrivedi/nexus-scheduler/gen/mocks/booking/mailer/risumailer"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
)

type mockRenderer struct {
	renderFunc func(name string, data any) (string, error)
}

func (m *mockRenderer) Render(name string, data any) (string, error) {
	if m.renderFunc != nil {
		return m.renderFunc(name, data)
	}

	return "<html>mock body</html>", nil
}

var _ templaterenderer.Renderer = (*mockRenderer)(nil)

func TestMailer_Send_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	sender := mocks.NewMockSender(ctrl)
	renderer := &mockRenderer{}

	email := entity.BookingEmail{
		Type:          entity.EmailBookingOnline,
		To:            "mock-recipient@example.com",
		ReferenceCode: "MOCK-123",
		Start:         time.Now(),
		End:           time.Now().Add(time.Hour),
	}

	sender.EXPECT().
		Send(
			gomock.Any(),
			"mock-from@example.com",
			email.To,
			email.Subject(),
			"<html>mock body</html>",
		).
		Return(nil)

	m := New(
		sender,
		renderer,
		"mock-from@example.com",
	)

	err := m.Send(context.Background(), email)

	require.NoError(t, err)
}

func TestMailer_Send_NotSendable(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	sender := mocks.NewMockSender(ctrl)
	renderer := &mockRenderer{}

	m := New(
		sender,
		renderer,
		"mock-from@example.com",
	)

	email := entity.BookingEmail{}

	err := m.Send(context.Background(), email)

	require.NoError(t, err)
}

func TestMailer_Send_ValidationError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	sender := mocks.NewMockSender(ctrl)
	renderer := &mockRenderer{}

	m := New(
		sender,
		renderer,
		"mock-from@example.com",
	)

	email := entity.BookingEmail{
		To: "mock-recipient@example.com",
	}

	err := m.Send(context.Background(), email)

	require.Error(t, err)
	require.Contains(t, err.Error(), "booking email validation")
}

func TestMailer_Send_RendererError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	sender := mocks.NewMockSender(ctrl)

	renderer := &mockRenderer{
		renderFunc: func(name string, data any) (string, error) {
			return "", errors.New("mock renderer error")
		},
	}

	m := New(
		sender,
		renderer,
		"mock-from@example.com",
	)

	email := entity.BookingEmail{
		Type:          entity.EmailBookingOnline,
		To:            "mock-recipient@example.com",
		ReferenceCode: "MOCK-123",
		Start:         time.Now(),
		End:           time.Now().Add(time.Hour),
	}

	err := m.Send(context.Background(), email)

	require.Error(t, err)
	require.Contains(t, err.Error(), "mock renderer error")
}

func TestMailer_Send_SenderError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	sender := mocks.NewMockSender(ctrl)
	renderer := &mockRenderer{}

	sender.EXPECT().
		Send(
			gomock.Any(),
			"mock-from@example.com",
			"mock-recipient@example.com",
			gomock.Any(),
			gomock.Any(),
		).
		Return(errors.New("mock sender error"))

	m := New(
		sender,
		renderer,
		"mock-from@example.com",
	)

	email := entity.BookingEmail{
		Type:          entity.EmailBookingOnline,
		To:            "mock-recipient@example.com",
		ReferenceCode: "MOCK-123",
		Start:         time.Now(),
		End:           time.Now().Add(time.Hour),
	}

	err := m.Send(context.Background(), email)

	require.Error(t, err)
	require.Contains(t, err.Error(), "mock sender error")
}
