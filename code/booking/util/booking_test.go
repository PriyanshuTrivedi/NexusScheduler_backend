package util

import (
	"testing"
	"time"

	"github.com/PriyanshuTrivedi/nexus-scheduler/code/booking/entity"
	bookingpb "github.com/PriyanshuTrivedi/nexus-scheduler/gen/idl/booking"

	"github.com/stretchr/testify/assert"
)

func TestCreateRequestToBooking(t *testing.T) {
	req := &bookingpb.CreateBookingRequest{
		UserId:     "u1",
		ResourceId: "r1",
		StartUnix:  1900000000,
		EndUnix:    1900003600,
		Title:      "ENT consult",
		Subtitle:   "Dr. Rakesh",
	}
	b := CreateRequestToBooking(req)

	assert.Equal(t, "u1", b.UserID)
	assert.Equal(t, "r1", b.ResourceID)
	assert.Equal(t, "ENT consult", b.Title)
	assert.Equal(t, time.Unix(1900000000, 0), b.Start)
	assert.Equal(t, time.Unix(1900003600, 0), b.End)
}

func TestBookingToCreateResponse(t *testing.T) {
	b := &entity.Booking{ReferenceCode: "NXS-ABC123", Status: entity.StatusConfirmed}
	resp := BookingToCreateResponse(b)

	assert.Equal(t, "NXS-ABC123", resp.ReferenceCode)
	assert.Equal(t, bookingpb.BookingStatus_BOOKING_STATUS_CONFIRMED, resp.Status)
}

func TestBookingToStatusResponse(t *testing.T) {
	b := &entity.Booking{
		ReferenceCode: "NXS-ABC123", UserID: "u1", ResourceID: "r1",
		Title: "ENT consult", Subtitle: "Dr. Rakesh", Status: entity.StatusCancelled,
		Start: time.Unix(1900000000, 0), End: time.Unix(1900003600, 0),
	}
	resp := BookingToStatusResponse(b)

	assert.Equal(t, "NXS-ABC123", resp.ReferenceCode)
	assert.Equal(t, bookingpb.BookingStatus_BOOKING_STATUS_CANCELLED, resp.Status)
	assert.Equal(t, int64(1900000000), resp.StartUnix)
}

func TestStatusToProto_AllCases(t *testing.T) {
	cases := map[entity.BookingStatus]bookingpb.BookingStatus{
		entity.StatusConfirmed:   bookingpb.BookingStatus_BOOKING_STATUS_CONFIRMED,
		entity.StatusWaitlisted:  bookingpb.BookingStatus_BOOKING_STATUS_WAITLISTED,
		entity.StatusCancelled:   bookingpb.BookingStatus_BOOKING_STATUS_CANCELLED,
		entity.StatusRescheduled: bookingpb.BookingStatus_BOOKING_STATUS_RESCHEDULED,
		entity.StatusFailed:      bookingpb.BookingStatus_BOOKING_STATUS_FAILED,
		entity.StatusUnspecified: bookingpb.BookingStatus_BOOKING_STATUS_UNSPECIFIED,
	}
	for in, want := range cases {
		got := BookingToCreateResponse(&entity.Booking{Status: in}).Status
		assert.Equal(t, want, got)
	}
}
