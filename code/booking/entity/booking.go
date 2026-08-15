package entity

import (
	"errors"
	"time"
)

type BookingStatus int

const (
	StatusUnspecified BookingStatus = iota
	StatusConfirmed
	StatusWaitlisted
	StatusCancelled
	StatusRescheduled
	StatusFailed
)

// String satisfies fmt.Stringer — fires automatically in logs/Printf,
// e.g. zap.Any("status", b.Status) prints "confirmed", not "1".
func (s BookingStatus) String() string {
	switch s {
	case StatusConfirmed:
		return "confirmed"
	case StatusWaitlisted:
		return "waitlisted"
	case StatusCancelled:
		return "cancelled"
	case StatusRescheduled:
		return "rescheduled"
	case StatusFailed:
		return "failed"
	default:
		return "unspecified"
	}
}

// ParseBookingStatus is the only place a raw DB string becomes a
// BookingStatus — store.go calls this after every SELECT.
func ParseBookingStatus(v string) BookingStatus {
	switch v {
	case "confirmed":
		return StatusConfirmed
	case "waitlisted":
		return StatusWaitlisted
	case "cancelled":
		return StatusCancelled
	case "rescheduled":
		return StatusRescheduled
	case "failed":
		return StatusFailed
	default:
		return StatusUnspecified
	}
}

var (
	ErrInvalidUserID     = errors.New("entities: user_id is required")
	ErrInvalidResourceID = errors.New("entities: resource_id is required")
	ErrInvalidTimeWindow = errors.New("entities: end time must be after start time")
	ErrInvalidTitle      = errors.New("entities: title is required")
	ErrPastStartTime     = errors.New("entities: start time must be in the future")
)

// Booking is the domain truth — no proto tags, no db tags. controller
// and store both operate on this; only util/ knows proto exists and
// only store/ knows SQL column names.
type Booking struct {
	ID              string
	ReferenceCode   string
	UserID          string
	UserEmail       string
	ResourceID      string
	Start           time.Time
	End             time.Time
	Title           string
	Subtitle        string
	Status          BookingStatus
	ParentBookingID string // set only when this row resulted from a reschedule
}

// Validate enforces invariants before this ever reaches the store layer.
func (b Booking) Validate() error {
	if b.UserID == "" {
		return ErrInvalidUserID
	}
	if b.ResourceID == "" {
		return ErrInvalidResourceID
	}
	if b.Title == "" {
		return ErrInvalidTitle
	}
	if !b.End.After(b.Start) {
		return ErrInvalidTimeWindow
	}
	if b.Start.Before(time.Now()) {
		return ErrPastStartTime
	}
	return nil
}

type WaitlistEntry struct {
	ID             string
	ResourceID     string
	UserID         string
	PreferredStart time.Time
	Position       int
}
