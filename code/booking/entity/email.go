package entity

import (
	"errors"
	"strings"
	"time"
)

type EmailType string

const (
	EmailBooking        EmailType = "booking"
	EmailBookingOnline  EmailType = "booking_online"
	EmailBookingOffline EmailType = "booking_offline"
	EmailCancellation   EmailType = "cancellation"
	EmailRescheduling   EmailType = "rescheduling"
)

type BookingEmail struct {
	Type          EmailType
	To            string
	RecipientName string
	ClientName    string
	ResourceName  string
	ReferenceCode string
	Title         string
	Subtitle      string
	Start         time.Time
	End           time.Time
	PreviousStart time.Time
	PreviousEnd   time.Time
	HasPrevious   bool
	MeetingMode   string
	Address       string
	MeetingLink   string
}

func (e BookingEmail) IsSendable() bool {
	return strings.TrimSpace(e.To) != ""
}

func (e BookingEmail) Validate() error {
	if !e.IsSendable() {
		return errors.New("email recipient is required")
	}

	if e.ReferenceCode == "" {
		return errors.New("email reference code is required")
	}

	if e.Start.IsZero() || e.End.IsZero() {
		return errors.New("email booking time is required")
	}

	if !e.End.After(e.Start) {
		return errors.New("email end time must be after start time")
	}

	return nil
}

func (e BookingEmail) Subject() string {
	switch e.Type {
	case EmailBookingOnline, EmailBookingOffline, EmailBooking:
		return "Nexus Scheduler - Booking Confirmed"
	case EmailCancellation:
		return "Nexus Scheduler - Booking Cancelled"
	case EmailRescheduling:
		return "Nexus Scheduler - Booking Rescheduled"
	default:
		return "Nexus Scheduler Notification"
	}
}

func (e BookingEmail) TemplateName() string {
	switch e.Type {
	case EmailBookingOnline:
		return "booking_online.html"
	case EmailBookingOffline:
		return "booking_offline.html"
	case EmailCancellation:
		return "cancellation.html"
	case EmailRescheduling:
		return "rescheduling.html"
	case EmailBooking:
		if strings.EqualFold(e.MeetingMode, "online") {
			return "booking_online.html"
		}
		return "booking_offline.html"
	default:
		return ""
	}
}

type BookingNotificationContext struct {
	ClientName    string
	ClientEmail   string
	ResourceName  string
	ResourceEmail string
	MeetingMode   string
	Address       string
	MeetingLink   string
	PreviousStart time.Time
	PreviousEnd   time.Time
	HasPrevious   bool
}
