package entity

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func validEmail() BookingEmail {
	return BookingEmail{
		Type: EmailBookingOnline, To: "user@example.com", ReferenceCode: "NXS-ABC123",
		Start: time.Date(2026, 9, 2, 15, 0, 0, 0, time.UTC), End: time.Date(2026, 9, 2, 16, 0, 0, 0, time.UTC),
	}
}

func TestBookingEmail_IsSendable(t *testing.T) {
	assert.True(t, validEmail().IsSendable())
	email := validEmail()
	email.To = "  "
	assert.False(t, email.IsSendable())
}

func TestBookingEmail_Validate(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*BookingEmail)
		want   string
	}{
		{"valid", func(*BookingEmail) {}, ""},
		{"missing recipient", func(e *BookingEmail) { e.To = "" }, "email recipient is required"},
		{"missing reference", func(e *BookingEmail) { e.ReferenceCode = "" }, "email reference code is required"},
		{"missing start", func(e *BookingEmail) { e.Start = time.Time{} }, "email booking time is required"},
		{"missing end", func(e *BookingEmail) { e.End = time.Time{} }, "email booking time is required"},
		{"end not after start", func(e *BookingEmail) { e.End = e.Start }, "email end time must be after start time"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := validEmail()
			tt.mutate(&e)
			err := e.Validate()
			if tt.want == "" {
				assert.NoError(t, err)
			} else {
				assert.EqualError(t, err, tt.want)
			}
		})
	}
}

func TestBookingEmail_Subject(t *testing.T) {
	tests := map[EmailType]string{
		EmailBooking:         "Nexus Scheduler - Booking Confirmed",
		EmailBookingOnline:   "Nexus Scheduler - Booking Confirmed",
		EmailBookingOffline:  "Nexus Scheduler - Booking Confirmed",
		EmailCancellation:    "Nexus Scheduler - Booking Cancelled",
		EmailRescheduling:    "Nexus Scheduler - Booking Rescheduled",
		EmailType("unknown"): "Nexus Scheduler Notification",
	}
	for typ, want := range tests {
		e := validEmail()
		e.Type = typ
		assert.Equal(t, want, e.Subject())
	}
}

func TestBookingEmail_TemplateName(t *testing.T) {
	tests := []struct {
		typ  EmailType
		mode string
		want string
	}{
		{EmailBookingOnline, "", "booking_online.html"},
		{EmailBookingOffline, "", "booking_offline.html"},
		{EmailCancellation, "", "cancellation.html"},
		{EmailRescheduling, "", "rescheduling.html"},
		{EmailBooking, "online", "booking_online.html"},
		{EmailBooking, "ONLINE", "booking_online.html"},
		{EmailBooking, "offline", "booking_offline.html"},
		{EmailBooking, "", "booking_offline.html"},
		{EmailType("unknown"), "online", ""},
	}
	for _, tt := range tests {
		e := validEmail()
		e.Type, e.MeetingMode = tt.typ, tt.mode
		assert.Equal(t, tt.want, e.TemplateName())
	}
}

func TestBookingEmail_ValidateTrimsRecipientOnlyForSendability(t *testing.T) {
	e := validEmail()
	e.To = "  user@example.com  "
	assert.True(t, e.IsSendable())
	assert.NoError(t, e.Validate())
	assert.True(t, strings.Contains(e.To, "user@example.com"))
}
