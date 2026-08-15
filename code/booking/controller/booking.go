package controller

import (
	"context"

	"github.com/PriyanshuTrivedi/nexus-scheduler/code/booking/client"
	"github.com/PriyanshuTrivedi/nexus-scheduler/code/booking/entity"
	"github.com/PriyanshuTrivedi/nexus-scheduler/code/booking/mailer"
	"github.com/PriyanshuTrivedi/nexus-scheduler/code/booking/store"
)

//go:generate mockgen -source=booking.go -destination=../../../gen/mocks/booking/controller/booking_mock.go -package=mocks

type Controller interface {
	CreateBooking(ctx context.Context, b entity.Booking) (*entity.Booking, error)
	CancelBooking(ctx context.Context, referenceCode string) (entity.BookingStatus, error)
	RescheduleBooking(ctx context.Context, referenceCode string, newStart, newEnd int64) (*entity.Booking, error)
	GetBooking(ctx context.Context, referenceCode string) (*entity.Booking, error)
	ListUserBookings(ctx context.Context, userID string, upcoming bool) ([]*entity.Booking, error)
	ListResourceBookings(ctx context.Context, resourceID string, upcoming bool) ([]*entity.Booking, error)
}

type controller struct {
	store  store.Store
	client client.Client
	mailer mailer.Mailer
}

func New(s store.Store, c client.Client, m mailer.Mailer) Controller {
	return &controller{store: s, client: c, mailer: m}
}

func (ctl *controller) CreateBooking(ctx context.Context, b entity.Booking) (*entity.Booking, error) {
	if err := b.Validate(); err != nil {
		return nil, err
	}

	release, ok, err := ctl.client.AcquireLock(ctx, b.ResourceID)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, store.ErrSlotAlreadyBooked
	}
	defer release()

	created, err := ctl.store.CreateBooking(ctx, b)
	if err != nil {
		return nil, err
	}
	_ = ctl.client.PublishEvent(ctx, "booking.created", created.ReferenceCode)
	if created.UserEmail != "" {
		if err := ctl.mailer.SendBookingConfirmation(ctx, created.UserEmail, "Client", created.ReferenceCode, created.ResourceID, created.Title, created.Start, created.End); err != nil {
			// Booking correctness must not depend on SMTP availability. The
			// confirmation is best-effort until a durable notification worker exists.
		}
	}
	return created, nil
}

func (ctl *controller) CancelBooking(ctx context.Context, referenceCode string) (entity.BookingStatus, error) {
	status, err := ctl.store.CancelBooking(ctx, referenceCode)
	if err != nil {
		return entity.StatusUnspecified, err
	}
	_ = ctl.client.PublishEvent(ctx, "booking.cancelled", referenceCode)
	return status, nil
}

func (ctl *controller) RescheduleBooking(ctx context.Context, referenceCode string, newStart, newEnd int64) (*entity.Booking, error) {
	b, err := ctl.store.RescheduleBooking(ctx, referenceCode, newStart, newEnd)
	if err != nil {
		return nil, err
	}
	_ = ctl.client.PublishEvent(ctx, "booking.rescheduled", b.ReferenceCode)
	return b, nil
}

func (ctl *controller) ListResourceBookings(ctx context.Context, resourceID string, upcoming bool) ([]*entity.Booking, error) {
	if resourceID == "" {
		return nil, entity.ErrInvalidResourceID
	}
	return ctl.store.ListResourceBookings(ctx, resourceID, upcoming)
}

func (ctl *controller) GetBooking(ctx context.Context, referenceCode string) (*entity.Booking, error) {
	return ctl.store.GetBooking(ctx, referenceCode)
}

func (ctl *controller) ListUserBookings(ctx context.Context, userID string, upcoming bool) ([]*entity.Booking, error) {
	if userID == "" {
		return nil, entity.ErrInvalidUserID
	}
	return ctl.store.ListUserBookings(ctx, userID, upcoming)
}
