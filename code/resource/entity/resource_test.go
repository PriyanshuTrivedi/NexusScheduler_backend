package entity

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestTenantTypeString(t *testing.T) {
	assert.Equal(t, "individual", TenantTypeIndividual.String())
	assert.Equal(t, "org", TenantTypeOrg.String())
	assert.Equal(t, "unspecified", TenantTypeUnspecified.String())
	assert.Equal(t, "unspecified", TenantType(99).String())
}

func TestParseTenantType(t *testing.T) {
	assert.Equal(t, TenantTypeIndividual, ParseTenantType("individual"))
	assert.Equal(t, TenantTypeOrg, ParseTenantType("org"))
	assert.Equal(t, TenantTypeUnspecified, ParseTenantType("invalid"))
}

func TestResourceTypeValidate(t *testing.T) {
	assert.NoError(t, (ResourceType{Name: "doctor"}).Validate())
	assert.ErrorIs(t, (ResourceType{}).Validate(), ErrInvalidResourceTypeName)
}

func TestMeetingMode(t *testing.T) {
	assert.Equal(t, "online", MeetingModeOnline.String())
	assert.Equal(t, "offline", MeetingModeOffline.String())
	assert.Equal(t, "hybrid", MeetingModeHybrid.String())
	assert.Equal(t, "unspecified", MeetingModeUnspecified.String())
	assert.Equal(t, "unspecified", MeetingMode(99).String())

	assert.True(t, MeetingModeOnline.Valid())
	assert.True(t, MeetingModeOffline.Valid())
	assert.True(t, MeetingModeHybrid.Valid())
	assert.False(t, MeetingModeUnspecified.Valid())
	assert.False(t, MeetingMode(99).Valid())

	assert.False(t, MeetingModeOnline.RequiresLocation())
	assert.True(t, MeetingModeOffline.RequiresLocation())
	assert.True(t, MeetingModeHybrid.RequiresLocation())
}

func TestDayOfWeekValid(t *testing.T) {
	assert.True(t, Monday.Valid())
	assert.True(t, Sunday.Valid())
	assert.False(t, DayUnspecified.Valid())
	assert.False(t, DayOfWeek(8).Valid())
}

func TestSlotStatusString(t *testing.T) {
	assert.Equal(t, "open", SlotStatusOpen.String())
	assert.Equal(t, "held", SlotStatusHeld.String())
	assert.Equal(t, "booked", SlotStatusBooked.String())
	assert.Equal(t, "blocked", SlotStatusBlocked.String())
	assert.Equal(t, "unspecified", SlotStatusUnspecified.String())
	assert.Equal(t, "unspecified", SlotStatus(99).String())
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
		{"minute too low", TimeSlot{9, -1, 10, 0}, ErrInvalidSlot},
		{"minute too high", TimeSlot{9, 60, 10, 0}, ErrInvalidSlot},
		{"end minute too high", TimeSlot{9, 0, 10, 60}, ErrInvalidSlot},
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

func TestSlotValidate(t *testing.T) {
	assert.NoError(t, (Slot{
		ResourceID: "res-1",
		Start:      timeFromUnix(1),
		End:        timeFromUnix(2),
	}).Validate())

	assert.ErrorIs(t, (Slot{ResourceID: ""}).Validate(), ErrInvalidResourceID)

	assert.ErrorIs(t, (Slot{
		ResourceID: "res-1",
		Start:      timeFromUnix(2),
		End:        timeFromUnix(1),
	}).Validate(), ErrInvalidTimeRange)
}

func timeFromUnix(v int64) time.Time {
	return time.Unix(v, 0)
}
