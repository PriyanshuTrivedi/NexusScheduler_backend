package entity

import (
	"errors"
	"time"
)

type TenantType int

const (
	TenantTypeUnspecified TenantType = iota
	TenantTypeIndividual
	TenantTypeOrg
)

func (t TenantType) String() string {
	switch t {
	case TenantTypeIndividual:
		return "individual"
	case TenantTypeOrg:
		return "org"
	default:
		return "unspecified"
	}
}

func ParseTenantType(v string) TenantType {
	switch v {
	case "individual":
		return TenantTypeIndividual
	case "org":
		return TenantTypeOrg
	default:
		return TenantTypeUnspecified
	}
}

type ResourceType struct {
	ID       string
	Name     string
	IsActive bool
}

func (r ResourceType) Validate() error {
	if r.Name == "" {
		return ErrInvalidResourceTypeName
	}
	return nil
}

type MeetingMode int

const (
	MeetingModeUnspecified MeetingMode = iota
	MeetingModeOnline
	MeetingModeOffline
	MeetingModeHybrid
)

func (m MeetingMode) String() string {
	switch m {
	case MeetingModeOnline:
		return "online"
	case MeetingModeOffline:
		return "offline"
	case MeetingModeHybrid:
		return "hybrid"
	default:
		return "unspecified"
	}
}

func (m MeetingMode) Valid() bool {
	return m >= MeetingModeOnline && m <= MeetingModeHybrid
}

func (m MeetingMode) RequiresLocation() bool {
	return m == MeetingModeOffline || m == MeetingModeHybrid
}

type DayOfWeek int

const (
	DayUnspecified DayOfWeek = iota
	Monday
	Tuesday
	Wednesday
	Thursday
	Friday
	Saturday
	Sunday
)

func (d DayOfWeek) Valid() bool {
	return d >= Monday && d <= Sunday
}

type SlotStatus int

const (
	SlotStatusUnspecified SlotStatus = iota
	SlotStatusOpen
	SlotStatusHeld
	SlotStatusBooked
	SlotStatusBlocked
)

func (s SlotStatus) String() string {
	switch s {
	case SlotStatusOpen:
		return "open"
	case SlotStatusHeld:
		return "held"
	case SlotStatusBooked:
		return "booked"
	case SlotStatusBlocked:
		return "blocked"
	default:
		return "unspecified"
	}
}

var (
	ErrInvalidName             = errors.New("entity: resource name is required")
	ErrInvalidUserID           = errors.New("entity: user_id is required")
	ErrInvalidResourceID       = errors.New("entity: resource_id is required")
	ErrInvalidResourceTypeID   = errors.New("entity: resource_type_id is required")
	ErrInvalidResourceTypeName = errors.New("entity: resource type name is required")
	ErrInvalidOrganizationID   = errors.New("entity: organization_id is required")
	ErrInvalidTenantType       = errors.New("entity: invalid tenant type")
	ErrInvalidStatus           = errors.New("entity: invalid status")
	ErrInvalidSlot             = errors.New("entity: invalid slot")
	ErrInvalidTimeRange        = errors.New("entity: end must be after start")
	ErrInvalidMeetingMode      = errors.New("entity: invalid meeting mode")
	ErrLocationRequired        = errors.New("entity: latitude/longitude are required when meeting_mode is OFFLINE or HYBRID")
	ErrInvalidDay              = errors.New("entity: invalid day_of_week")
	ErrInvalidTimezone         = errors.New("entity: timezone is not a recognized IANA name")
	ErrEmptyRecurrenceRule     = errors.New("entity: recurrence rule must have at least one time slot")
	ErrOverlappingSlots        = errors.New("entity: time slots within a recurrence rule must not overlap")
)

type TimeSlot struct {
	StartHour   int
	StartMinute int
	EndHour     int
	EndMinute   int
}

func (t TimeSlot) Validate() error {
	if t.StartHour < 0 || t.StartHour > 23 || t.EndHour < 0 || t.EndHour > 23 ||
		t.StartMinute < 0 || t.StartMinute > 59 || t.EndMinute < 0 || t.EndMinute > 59 {
		return ErrInvalidSlot
	}
	start := t.StartHour*60 + t.StartMinute
	end := t.EndHour*60 + t.EndMinute
	if start >= end {
		return ErrInvalidSlot
	}
	return nil
}

type RecurrenceRule struct {
	Day      DayOfWeek
	Timezone string
	Slots    []TimeSlot
}

func (r RecurrenceRule) Validate() error {
	if !r.Day.Valid() {
		return ErrInvalidDay
	}
	if len(r.Slots) == 0 {
		return ErrEmptyRecurrenceRule
	}
	if _, err := time.LoadLocation(r.Timezone); err != nil {
		return ErrInvalidTimezone
	}
	for _, ts := range r.Slots {
		if err := ts.Validate(); err != nil {
			return err
		}
	}
	for i := 0; i < len(r.Slots); i++ {
		for j := i + 1; j < len(r.Slots); j++ {
			a, b := r.Slots[i], r.Slots[j]
			aStart, aEnd := a.StartHour*60+a.StartMinute, a.EndHour*60+a.EndMinute
			bStart, bEnd := b.StartHour*60+b.StartMinute, b.EndHour*60+b.EndMinute
			if aStart < bEnd && bStart < aEnd {
				return ErrOverlappingSlots
			}
		}
	}
	return nil
}

type Resource struct {
	ID             string
	TenantType     TenantType
	OrgID          string
	UserID         string
	ResourceTypeID string
	Name           string
	MeetingMode    MeetingMode
	Latitude       *float64
	Longitude      *float64
	Attributes     map[string]string
	Recurrence     []RecurrenceRule
	IsActive       bool
}

func (r Resource) Validate() error {
	if r.TenantType == TenantTypeUnspecified {
		return ErrInvalidTenantType
	}
	if r.TenantType == TenantTypeIndividual && r.OrgID != "" {
		return ErrInvalidOrganizationID
	}
	if r.TenantType == TenantTypeOrg && r.OrgID == "" {
		return ErrInvalidOrganizationID
	}
	if r.ResourceTypeID == "" {
		return ErrInvalidResourceTypeID
	}
	if r.Name == "" {
		return ErrInvalidName
	}
	if !r.MeetingMode.Valid() {
		return ErrInvalidMeetingMode
	}
	if r.MeetingMode.RequiresLocation() && (r.Latitude == nil || r.Longitude == nil) {
		return ErrLocationRequired
	}
	for _, rule := range r.Recurrence {
		if err := rule.Validate(); err != nil {
			return err
		}
	}
	return nil
}

type Slot struct {
	ID         string
	ResourceID string
	Start      time.Time
	End        time.Time
	Status     SlotStatus
}

func (s Slot) Validate() error {
	if s.ResourceID == "" {
		return ErrInvalidResourceID
	}
	if !s.End.After(s.Start) {
		return ErrInvalidTimeRange
	}
	return nil
}

type SearchResourceRequest struct {
	TenantType     TenantType
	OrgID          string
	Name           string
	ResourceTypeID string
	MeetingMode    MeetingMode
	Attributes     map[string]string
	Latitude       float64
	Longitude      float64
	RadiusKM       float64
	WindowStart    time.Time
	WindowEnd      time.Time
}

type ResourceSummary struct {
	ResourceID         string
	TenantType         TenantType
	OrgID              string
	Name               string
	ResourceType       ResourceType
	MeetingMode        MeetingMode
	DistanceKM         float64
	Attributes         map[string]string
	IsActive           bool
	NextAvailableSlots []Slot
}

type LeavePeriod struct {
	ResourceID string
	Start      time.Time
	End        time.Time
	Reason     string
}

type SlotException struct {
	ResourceID string
	Start      time.Time
	End        time.Time
	Reason     string
}
