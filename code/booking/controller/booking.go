package controller

import (
	"context"
	"log"
	"time"

	"github.com/PriyanshuTrivedi/nexus-scheduler/code/booking/client"
	"github.com/PriyanshuTrivedi/nexus-scheduler/code/booking/entity"
	"github.com/PriyanshuTrivedi/nexus-scheduler/code/booking/mailer"
	"github.com/PriyanshuTrivedi/nexus-scheduler/code/booking/store"
	"github.com/PriyanshuTrivedi/nexus-scheduler/code/booking/util"
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
	return &controller{
		store:  s,
		client: c,
		mailer: m,
	}
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

	if notification, ok := util.BookingNotificationContextFromContext(ctx); ok {
		emailType := entity.EmailBookingOffline
		if notification.MeetingMode == "online" {
			emailType = entity.EmailBookingOnline
		}

		ctl.sendEmails(util.BuildBookingEmails(created, notification, emailType))
	}

	return created, nil
}

func (ctl *controller) CancelBooking(ctx context.Context, referenceCode string) (entity.BookingStatus, error) {
	notification, hasNotification := util.BookingNotificationContextFromContext(ctx)

	var booking *entity.Booking

	if hasNotification {
		var err error
		booking, err = ctl.store.GetBooking(ctx, referenceCode)
		if err != nil {
			return entity.StatusUnspecified, err
		}
	}

	status, err := ctl.store.CancelBooking(ctx, referenceCode)
	if err != nil {
		return entity.StatusUnspecified, err
	}

	_ = ctl.client.PublishEvent(ctx, "booking.cancelled", referenceCode)

	if hasNotification && booking != nil {
		ctl.sendEmails(util.BuildBookingEmails(booking, notification, entity.EmailCancellation))
	}

	return status, nil
}

func (ctl *controller) RescheduleBooking(ctx context.Context, referenceCode string, newStart, newEnd int64) (*entity.Booking, error) {
	notification, hasNotification := util.BookingNotificationContextFromContext(ctx)

	var previous *entity.Booking

	if hasNotification {
		var err error
		previous, err = ctl.store.GetBooking(ctx, referenceCode)
		if err != nil {
			return nil, err
		}
	}

	b, err := ctl.store.RescheduleBooking(ctx, referenceCode, newStart, newEnd)
	if err != nil {
		return nil, err
	}

	b.Start = time.Unix(newStart, 0)
	b.End = time.Unix(newEnd, 0)

	_ = ctl.client.PublishEvent(ctx, "booking.rescheduled", b.ReferenceCode)

	if hasNotification && previous != nil {
		notification.PreviousStart = previous.Start
		notification.PreviousEnd = previous.End
		notification.HasPrevious = true

		ctl.sendEmails(util.BuildBookingEmails(b, notification, entity.EmailRescheduling))
	}

	return b, nil
}

func (ctl *controller) sendEmails(emails []entity.BookingEmail) {
	for _, email := range emails {
		go func(email entity.BookingEmail) {
			ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			defer cancel()

			if err := ctl.mailer.Send(ctx, email); err != nil {
				log.Printf(
					"booking email failed: type=%s to=%s reference=%s error=%v",
					email.Type,
					email.To,
					email.ReferenceCode,
					err,
				)
			}
		}(email)
	}
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
