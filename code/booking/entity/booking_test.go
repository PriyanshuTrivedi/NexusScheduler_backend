package entity

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func validBooking() Booking {
	return Booking{
		UserID:     "u1",
		ResourceID: "r1",
		Title:      "ENT consult",
		Start:      time.Now().Add(time.Hour),
		End:        time.Now().Add(2 * time.Hour),
	}
}

func TestValidate_Success(t *testing.T) {
	assert.NoError(t, validBooking().Validate())
}

func TestValidate_MissingUserID(t *testing.T) {
	b := validBooking()
	b.UserID = ""
	assert.ErrorIs(t, b.Validate(), ErrInvalidUserID)
}

func TestValidate_MissingResourceID(t *testing.T) {
	b := validBooking()
	b.ResourceID = ""
	assert.ErrorIs(t, b.Validate(), ErrInvalidResourceID)
}

func TestValidate_MissingTitle(t *testing.T) {
	b := validBooking()
	b.Title = ""
	assert.ErrorIs(t, b.Validate(), ErrInvalidTitle)
}

func TestValidate_EndBeforeStart(t *testing.T) {
	b := validBooking()
	b.End = b.Start.Add(-time.Hour)
	assert.ErrorIs(t, b.Validate(), ErrInvalidTimeWindow)
}

func TestValidate_PastStartTime(t *testing.T) {
	b := validBooking()
	b.Start = time.Now().Add(-time.Hour)
	b.End = time.Now()
	assert.ErrorIs(t, b.Validate(), ErrPastStartTime)
}

func TestBookingStatus_StringRoundTrip(t *testing.T) {
	cases := []BookingStatus{StatusConfirmed, StatusWaitlisted, StatusCancelled, StatusRescheduled, StatusFailed}
	for _, s := range cases {
		assert.Equal(t, s, ParseBookingStatus(s.String()))
	}
}

func TestParseBookingStatus_Unknown(t *testing.T) {
	assert.Equal(t, StatusUnspecified, ParseBookingStatus("garbage"))
}
