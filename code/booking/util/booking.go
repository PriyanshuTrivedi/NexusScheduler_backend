package util

import (
	"time"

	"github.com/PriyanshuTrivedi/nexus-scheduler/code/booking/entity"
	bookingpb "github.com/PriyanshuTrivedi/nexus-scheduler/gen/idl/booking"
)

func statusToProto(s entity.BookingStatus) bookingpb.BookingStatus {
	switch s {
	case entity.StatusConfirmed:
		return bookingpb.BookingStatus_BOOKING_STATUS_CONFIRMED
	case entity.StatusWaitlisted:
		return bookingpb.BookingStatus_BOOKING_STATUS_WAITLISTED
	case entity.StatusCancelled:
		return bookingpb.BookingStatus_BOOKING_STATUS_CANCELLED
	case entity.StatusRescheduled:
		return bookingpb.BookingStatus_BOOKING_STATUS_RESCHEDULED
	case entity.StatusFailed:
		return bookingpb.BookingStatus_BOOKING_STATUS_FAILED
	default:
		return bookingpb.BookingStatus_BOOKING_STATUS_UNSPECIFIED
	}
}

// CreateRequestToBooking converts the wire request into a domain entity.
// It does NOT validate — Booking.Validate() is called explicitly by the
// handler, so validation logic lives in exactly one place (entity).
func CreateRequestToBooking(req *bookingpb.CreateBookingRequest) entity.Booking {
	return entity.Booking{
		UserID:     req.GetUserId(),
		UserEmail:  req.GetUserEmail(),
		ResourceID: req.GetResourceId(),
		Start:      time.Unix(req.GetStartUnix(), 0),
		End:        time.Unix(req.GetEndUnix(), 0),
		Title:      req.GetTitle(),
		Subtitle:   req.GetSubtitle(),
	}
}

func BookingToCreateResponse(b *entity.Booking) *bookingpb.CreateBookingResponse {
	return &bookingpb.CreateBookingResponse{
		ReferenceCode: b.ReferenceCode,
		Status:        statusToProto(b.Status),
	}
}

func BookingToRescheduleResponse(b *entity.Booking) *bookingpb.RescheduleBookingResponse {
	return &bookingpb.RescheduleBookingResponse{
		ReferenceCode: b.ReferenceCode,
		Status:        statusToProto(b.Status),
	}
}

func BookingToStatusResponse(b *entity.Booking) *bookingpb.GetBookingStatusResponse {
	return &bookingpb.GetBookingStatusResponse{
		ReferenceCode: b.ReferenceCode,
		UserId:        b.UserID,
		ResourceId:    b.ResourceID,
		StartUnix:     b.Start.Unix(),
		EndUnix:       b.End.Unix(),
		Title:         b.Title,
		Subtitle:      b.Subtitle,
		Status:        statusToProto(b.Status),
	}
}

func CancelResponse(status entity.BookingStatus) *bookingpb.CancelBookingResponse {
	return &bookingpb.CancelBookingResponse{Status: statusToProto(status)}
}
