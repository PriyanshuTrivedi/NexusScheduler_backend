package entity

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestResourceTypeValidate(t *testing.T) {
	assert.NoError(t, (ResourceType{Name: "doctor"}).Validate())
	assert.ErrorIs(t, (ResourceType{}).Validate(), ErrInvalidResourceTypeName)
}

func TestTimeSlotValidate(t *testing.T) {
	cases := []struct {
		name string
		slot TimeSlot
		err  error
	}{
		{"valid", TimeSlot{9, 0, 17, 0}, nil},
		{"end before start", TimeSlot{17, 0, 9, 0}, ErrInvalidSlot},
		{"equal start and end", TimeSlot{9, 0, 9, 0}, ErrInvalidSlot},
		{"hour too low", TimeSlot{-1, 0, 10, 0}, ErrInvalidSlot},
		{"hour too high", TimeSlot{9, 0, 24, 0}, ErrInvalidSlot},
		{"minute too high", TimeSlot{9, 60, 10, 0}, ErrInvalidSlot},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.slot.Validate()
			if tc.err == nil {
				assert.NoError(t, err)
			} else {
				assert.ErrorIs(t, err, tc.err)
			}
		})
	}
}

func TestRecurrenceRuleValidate(t *testing.T) {
	valid := TimeSlot{9, 0, 17, 0}
	cases := []struct {
		name string
		rule RecurrenceRule
		err  error
	}{
		{"valid", RecurrenceRule{Day: Monday, Timezone: "Asia/Kolkata", Slots: []TimeSlot{valid}}, nil},
		{"unspecified day", RecurrenceRule{Day: DayUnspecified, Timezone: "Asia/Kolkata", Slots: []TimeSlot{valid}}, ErrInvalidDay},
		{"day out of range", RecurrenceRule{Day: DayOfWeek(8), Timezone: "Asia/Kolkata", Slots: []TimeSlot{valid}}, ErrInvalidDay},
		{"no slots", RecurrenceRule{Day: Monday, Timezone: "Asia/Kolkata"}, ErrEmptyRecurrenceRule},
		{"bad timezone", RecurrenceRule{Day: Monday, Timezone: "Not/AZone", Slots: []TimeSlot{valid}}, ErrInvalidTimezone},
		{"invalid slot", RecurrenceRule{Day: Monday, Timezone: "Asia/Kolkata", Slots: []TimeSlot{{17, 0, 9, 0}}}, ErrInvalidSlot},
		{"overlap", RecurrenceRule{Day: Monday, Timezone: "Asia/Kolkata", Slots: []TimeSlot{{9, 0, 12, 0}, {11, 0, 13, 0}}}, ErrOverlappingSlots},
		{"adjacent", RecurrenceRule{Day: Monday, Timezone: "Asia/Kolkata", Slots: []TimeSlot{{9, 0, 12, 0}, {12, 0, 13, 0}}}, nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.rule.Validate()
			if tc.err == nil {
				assert.NoError(t, err)
			} else {
				assert.ErrorIs(t, err, tc.err)
			}
		})
	}
}

func TestResourceValidate(t *testing.T) {
	lat, lng := 12.9716, 77.5946
	base := func() Resource {
		return Resource{TenantType: TenantTypeOrg, OrgID: "org-1", ResourceTypeID: "type-1", Name: "Dr. Rakesh", MeetingMode: MeetingModeOnline}
	}
	cases := []struct {
		name   string
		mutate func(Resource) Resource
		err    error
	}{
		{"valid", func(r Resource) Resource { return r }, nil},
		{"individual resource", func(r Resource) Resource { r.TenantType = TenantTypeIndividual; r.OrgID = ""; return r }, nil},
		{"missing type id", func(r Resource) Resource { r.ResourceTypeID = ""; return r }, ErrInvalidResourceTypeID},
		{"missing name", func(r Resource) Resource { r.Name = ""; return r }, ErrInvalidName},
		{"unspecified meeting mode", func(r Resource) Resource { r.MeetingMode = MeetingModeUnspecified; return r }, ErrInvalidMeetingMode},
		{"offline without location", func(r Resource) Resource { r.MeetingMode = MeetingModeOffline; return r }, ErrLocationRequired},
		{"hybrid without location", func(r Resource) Resource { r.MeetingMode = MeetingModeHybrid; return r }, ErrLocationRequired},
		{"offline with location", func(r Resource) Resource {
			r.MeetingMode = MeetingModeOffline
			r.Latitude, r.Longitude = &lat, &lng
			return r
		}, nil},
		{"invalid recurrence", func(r Resource) Resource {
			r.Recurrence = []RecurrenceRule{{Day: DayUnspecified, Timezone: "Asia/Kolkata", Slots: []TimeSlot{{9, 0, 17, 0}}}}
			return r
		}, ErrInvalidDay},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.mutate(base()).Validate()
			if tc.err == nil {
				assert.NoError(t, err)
			} else {
				assert.ErrorIs(t, err, tc.err)
			}
		})
	}
}

func TestSlotValidate(t *testing.T) {
	assert.ErrorIs(t, (Slot{ResourceID: ""}).Validate(), ErrInvalidResourceID)
}
