package util

import (
	"context"
	"errors"
	"strings"

	"github.com/PriyanshuTrivedi/nexus-scheduler/code/booking/entity"
	bookingpb "github.com/PriyanshuTrivedi/nexus-scheduler/gen/idl/booking"
	"google.golang.org/grpc/metadata"
)

type bookingNotificationContextKey struct{}

func WithBookingNotificationContext(ctx context.Context) context.Context {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return ctx
	}

	notification := entity.BookingNotificationContext{
		ClientName:    firstMetadataValue(md, "x-booking-client-name"),
		ClientEmail:   firstMetadataValue(md, "x-booking-client-email"),
		ResourceName:  firstMetadataValue(md, "x-booking-resource-name"),
		ResourceEmail: firstMetadataValue(md, "x-booking-resource-email"),
		MeetingMode:   firstMetadataValue(md, "x-booking-meeting-mode"),
		Address:       firstMetadataValue(md, "x-booking-address"),
		MeetingLink:   firstMetadataValue(md, "x-booking-meeting-link"),
	}

	return context.WithValue(ctx, bookingNotificationContextKey{}, notification)
}

func BookingNotificationContextFromContext(ctx context.Context) (entity.BookingNotificationContext, bool) {
	value, ok := ctx.Value(bookingNotificationContextKey{}).(entity.BookingNotificationContext)
	return value, ok
}

func BuildBookingEmails(booking *entity.Booking, notification entity.BookingNotificationContext, emailType entity.EmailType) []entity.BookingEmail {
	recipients := []struct {
		email string
		name  string
	}{
		{
			email: notification.ClientEmail,
			name:  notification.ClientName,
		},
		{
			email: notification.ResourceEmail,
			name:  notification.ResourceName,
		},
	}

	emails := make([]entity.BookingEmail, 0, len(recipients))
	sent := make(map[string]struct{})

	for _, recipient := range recipients {
		email := strings.TrimSpace(recipient.email)
		if email == "" {
			continue
		}

		if _, exists := sent[email]; exists {
			continue
		}

		sent[email] = struct{}{}

		emails = append(emails, entity.BookingEmail{
			Type:          emailType,
			To:            email,
			RecipientName: recipient.name,
			ClientName:    notification.ClientName,
			ResourceName:  notification.ResourceName,
			ReferenceCode: booking.ReferenceCode,
			Title:         booking.Title,
			Subtitle:      booking.Subtitle,
			Start:         booking.Start,
			End:           booking.End,
			PreviousStart: notification.PreviousStart,
			PreviousEnd:   notification.PreviousEnd,
			HasPrevious:   notification.HasPrevious,
			MeetingMode:   notification.MeetingMode,
			Address:       notification.Address,
			MeetingLink:   notification.MeetingLink,
		})
	}

	return emails
}

func IsValidationErr(err error) bool {
	return errors.Is(err, entity.ErrInvalidUserID) ||
		errors.Is(err, entity.ErrInvalidResourceID) ||
		errors.Is(err, entity.ErrInvalidTimeWindow) ||
		errors.Is(err, entity.ErrInvalidTitle) ||
		errors.Is(err, entity.ErrPastStartTime)
}

func BookingsToStatusResponse(bookings []*entity.Booking) *bookingpb.ListUserBookingsResponse {
	response := &bookingpb.ListUserBookingsResponse{
		Bookings: make([]*bookingpb.GetBookingStatusResponse, 0, len(bookings)),
	}

	for _, booking := range bookings {
		response.Bookings = append(response.Bookings, BookingToStatusResponse(booking))
	}

	return response
}

func firstMetadataValue(md metadata.MD, key string) string {
	values := md.Get(key)
	if len(values) == 0 {
		return ""
	}

	return values[0]
}
