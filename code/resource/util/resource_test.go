package util

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/PriyanshuTrivedi/nexus-scheduler/code/resource/entity"
	pb "github.com/PriyanshuTrivedi/nexus-scheduler/gen/idl/resource"
)

func TestResourceTypeRoundTrip(t *testing.T) {
	rt := entity.ResourceType{ID: "type-1", Name: "doctor"}
	got := ResourceTypeToProto(rt)
	assert.Equal(t, "type-1", got.ResourceTypeId)
	assert.Equal(t, "doctor", got.Name)
}

func TestRecurrenceRulesRoundTrip(t *testing.T) {
	in := []*pb.RecurrenceRule{{Day: pb.DayOfWeek_DAY_OF_WEEK_MONDAY, Timezone: "Asia/Kolkata", Slots: []*pb.TimeSlot{{StartHour: 9, EndHour: 17}}}}
	rules := RecurrenceRulesFromProto(in)
	assert.Len(t, rules, 1)
	assert.Equal(t, entity.Monday, rules[0].Day)
	assert.Equal(t, "Asia/Kolkata", rules[0].Timezone)
	assert.Equal(t, 9, rules[0].Slots[0].StartHour)
	back := recurrenceToProto(rules)
	assert.Equal(t, pb.DayOfWeek_DAY_OF_WEEK_MONDAY, back[0].Day)
	assert.EqualValues(t, 17, back[0].Slots[0].EndHour)
}

func TestResourceFromCreateRequest(t *testing.T) {
	lat, lng := 12.9716, 77.5946
	req := &pb.CreateResourceRequest{TenantType: pb.TenantType_TENANT_TYPE_ORG, OrgId: "org-1", ResourceTypeId: "type-1", Name: "Dr. Rakesh", MeetingMode: pb.MeetingMode_MEETING_MODE_OFFLINE, Lat: &lat, Lng: &lng, Attributes: map[string]string{"department": "ENT"}}
	r := ResourceFromCreateRequest(req)
	assert.Equal(t, "org-1", r.OrgID)
	assert.Equal(t, "type-1", r.ResourceTypeID)
	assert.True(t, r.IsActive)
	assert.Equal(t, "ENT", r.Attributes["department"])
	assert.Equal(t, lat, *r.Latitude)
	assert.Equal(t, lng, *r.Longitude)
}

func TestResourceFromCreateRequest_OptionalLocationStaysNil(t *testing.T) {
	r := ResourceFromCreateRequest(&pb.CreateResourceRequest{TenantType: pb.TenantType_TENANT_TYPE_ORG, OrgId: "org-1", ResourceTypeId: "type-1", Name: "Backend Panel", MeetingMode: pb.MeetingMode_MEETING_MODE_ONLINE})
	assert.Nil(t, r.Latitude)
	assert.Nil(t, r.Longitude)
}

func TestSearchRequestFromProto(t *testing.T) {
	start := time.Date(2026, 8, 10, 0, 0, 0, 0, time.UTC).Unix()
	end := time.Date(2026, 8, 17, 0, 0, 0, 0, time.UTC).Unix()
	req := &pb.SearchResourcesRequest{TenantType: pb.TenantType_TENANT_TYPE_ORG, OrgId: "org-1", ResourceTypeId: "type-1", WindowStartUnix: start, WindowEndUnix: end}
	sr := SearchRequestFromProto(req)
	assert.Equal(t, "type-1", sr.ResourceTypeID)
	assert.Equal(t, start, sr.WindowStart.Unix())
	assert.Equal(t, end, sr.WindowEnd.Unix())
}

func TestSearchRequestFromProto_ZeroWindowMeansNoFilter(t *testing.T) {
	sr := SearchRequestFromProto(&pb.SearchResourcesRequest{TenantType: pb.TenantType_TENANT_TYPE_ORG, OrgId: "org-1"})
	assert.True(t, sr.WindowStart.IsZero())
	assert.True(t, sr.WindowEnd.IsZero())
}

func TestSlotExceptionFromProto(t *testing.T) {
	se := SlotExceptionFromProto(&pb.AddSlotExceptionRequest{ResourceId: "res-1", StartUnix: 1000, EndUnix: 2000, Reason: "extra coverage"})
	assert.Equal(t, "res-1", se.ResourceID)
	assert.Equal(t, int64(1000), se.Start.Unix())
	assert.Equal(t, int64(2000), se.End.Unix())
}

func TestLeavePeriodFromProto(t *testing.T) {
	lp := LeavePeriodFromProto(&pb.SetLeavePeriodRequest{ResourceId: "res-1", StartDayUnix: 1000, EndDayUnix: 2000, Reason: "vacation"})
	assert.Equal(t, "res-1", lp.ResourceID)
	assert.Equal(t, int64(1000), lp.Start.Unix())
	assert.Equal(t, int64(2000), lp.End.Unix())
}

func TestSearchResponseToProto(t *testing.T) {
	resp := SearchResponseToProto([]entity.ResourceSummary{{
		ResourceID: "res-1", TenantType: entity.TenantTypeOrg, OrgID: "org-1", Name: "Dr. Rakesh", ResourceType: entity.ResourceType{ID: "type-1", Name: "doctor"},
		MeetingMode: entity.MeetingModeOffline, DistanceKM: 2.5, Attributes: map[string]string{"department": "ENT"}, IsActive: true,
		NextAvailableSlots: []entity.Slot{{Start: time.Unix(1000, 0), End: time.Unix(2000, 0)}},
	}})
	got := resp.Resources[0]
	assert.Equal(t, "res-1", got.ResourceId)
	assert.Equal(t, "type-1", got.ResourceType.ResourceTypeId)
	assert.Equal(t, "doctor", got.ResourceType.Name)
	assert.True(t, got.IsActive)
	assert.Equal(t, 2.5, got.DistanceKm)
}

func TestSlotToProto(t *testing.T) {
	got := SlotToProto(entity.Slot{ID: "slot-1", ResourceID: "res-1", Start: time.Unix(1000, 0), End: time.Unix(2000, 0), Status: entity.SlotStatusOpen})
	assert.Equal(t, "slot-1", got.SlotId)
	assert.Equal(t, "res-1", got.ResourceId)
	assert.Equal(t, pb.SlotStatus_SLOT_STATUS_OPEN, got.Status)
}

func TestResponseWrappers(t *testing.T) {
	assert.Equal(t, "res-1", CreateResourceResponseToProto("res-1").ResourceId)
	assert.EqualValues(t, 5, SetRecurringAvailabilityResponseToProto(5).SlotsGenerated)
	assert.Equal(t, "slot-1", AddSlotExceptionResponseToProto("slot-1", entity.SlotStatusOpen).SlotId)
	assert.Equal(t, pb.SlotStatus_SLOT_STATUS_BLOCKED, RemoveSlotExceptionResponseToProto(entity.SlotStatusBlocked).Status)
	assert.EqualValues(t, 3, SetLeavePeriodResponseToProto(3).SlotsRemoved)
}
