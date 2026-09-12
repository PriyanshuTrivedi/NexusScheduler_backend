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

	assert.Equal(t, "type-1", got.GetResourceTypeId())
	assert.Equal(t, "doctor", got.GetName())
}

func TestRecurrenceRulesRoundTrip(t *testing.T) {
	in := []*pb.RecurrenceRule{{
		Day:      pb.DayOfWeek_DAY_OF_WEEK_MONDAY,
		Timezone: "Asia/Kolkata",
		Slots:    []*pb.TimeSlot{{StartHour: 9, EndHour: 17}},
	}}

	rules := RecurrenceRulesFromProto(in)

	assert.Len(t, rules, 1)
	assert.Equal(t, entity.Monday, rules[0].Day)
	assert.Equal(t, "Asia/Kolkata", rules[0].Timezone)
	assert.Len(t, rules[0].Slots, 1)
	assert.Equal(t, 9, rules[0].Slots[0].StartHour)

	back := recurrenceToProto(rules)

	assert.Len(t, back, 1)
	assert.Equal(t, pb.DayOfWeek_DAY_OF_WEEK_MONDAY, back[0].GetDay())
	assert.Equal(t, int32(17), back[0].GetSlots()[0].GetEndHour())
}

func TestResourceFromCreateRequest(t *testing.T) {
	req := &pb.CreateResourceRequest{
		UserId:         "user-1",
		TenantType:     pb.TenantType_TENANT_TYPE_ORG,
		OrgId:          ToStringPtr("org-1"),
		ResourceTypeId: "type-1",
		Name:           "Dr. Rakesh",
		MeetingMode:    pb.MeetingMode_MEETING_MODE_OFFLINE,
		Address:        ToStringPtr("MG Road, Bangalore"),
		Attributes:     map[string]string{"department": "ENT"},
	}

	r := ResourceFromCreateRequest(req)

	assert.Equal(t, "user-1", r.UserID)
	assert.Equal(t, entity.TenantTypeOrg, r.TenantType)
	assert.Equal(t, "org-1", *r.OrgID)
	assert.Equal(t, "type-1", r.ResourceType.ID)
	assert.Equal(t, "Dr. Rakesh", r.Name)
	assert.Equal(t, entity.MeetingModeOffline, r.MeetingMode)
	assert.Equal(t, "MG Road, Bangalore", *r.Address)
	assert.Equal(t, "ENT", r.Attributes["department"])
	assert.True(t, r.IsActive)
}

func TestResourceFromCreateRequest_OptionalAddressStaysNil(t *testing.T) {
	req := &pb.CreateResourceRequest{
		UserId:         "user-1",
		TenantType:     pb.TenantType_TENANT_TYPE_ORG,
		OrgId:          ToStringPtr("org-1"),
		ResourceTypeId: "type-1",
		Name:           "Backend Panel",
		MeetingMode:    pb.MeetingMode_MEETING_MODE_ONLINE,
	}

	r := ResourceFromCreateRequest(req)

	assert.Nil(t, r.Address)
}

func TestResourceFromUpdateRequest(t *testing.T) {
	req := &pb.UpdateResourceRequest{
		ResourceId:  "res-1",
		Name:        "Dr. Rakesh Updated",
		TenantType:  pb.TenantType_TENANT_TYPE_ORG,
		OrgId:       ToStringPtr("org-1"),
		MeetingMode: pb.MeetingMode_MEETING_MODE_OFFLINE,
		Address:     ToStringPtr("Indiranagar, Bangalore"),
		Attributes:  map[string]string{"department": "ENT"},
	}

	r := ResourceFromUpdateRequest(req)

	assert.Equal(t, "res-1", r.ID)
	assert.Equal(t, entity.TenantTypeOrg, r.TenantType)
	assert.Equal(t, "Dr. Rakesh Updated", r.Name)
	assert.Equal(t, "org-1", *r.OrgID)
	assert.Equal(t, entity.MeetingModeOffline, r.MeetingMode)
	assert.Equal(t, "Indiranagar, Bangalore", *r.Address)
	assert.Equal(t, "ENT", r.Attributes["department"])
}

func TestResourceFromUpdateRequest_OptionalAddressStaysNil(t *testing.T) {
	req := &pb.UpdateResourceRequest{
		ResourceId:  "res-1",
		Name:        "Dr. Rakesh",
		TenantType:  pb.TenantType_TENANT_TYPE_ORG,
		OrgId:       ToStringPtr("org-1"),
		MeetingMode: pb.MeetingMode_MEETING_MODE_ONLINE,
	}

	r := ResourceFromUpdateRequest(req)

	assert.Equal(t, "res-1", r.ID)
	assert.Nil(t, r.Address)
}

func TestSearchRequestFromProto(t *testing.T) {
	start := time.Date(2026, 8, 10, 0, 0, 0, 0, time.UTC).Unix()
	end := time.Date(2026, 8, 17, 0, 0, 0, 0, time.UTC).Unix()

	req := &pb.SearchResourcesRequest{
		TenantType:      pb.TenantType_TENANT_TYPE_ORG.Enum(),
		OrgId:           ToStringPtr("org-1"),
		Name:            ToStringPtr("Rakesh"),
		ResourceTypeId:  "type-1",
		MeetingMode:     pb.MeetingMode_MEETING_MODE_OFFLINE.Enum(),
		Lat:             ToFloatPtr(12.9716),
		Lng:             ToFloatPtr(77.5946),
		RadiusKm:        ToFloatPtr(10),
		WindowStartUnix: &start,
		WindowEndUnix:   &end,
	}

	sr := SearchRequestFromProto(req)

	assert.Equal(t, entity.TenantTypeOrg, *sr.TenantType)
	assert.Equal(t, "org-1", *sr.OrgID)
	assert.Equal(t, "Rakesh", *sr.Name)
	assert.Equal(t, "type-1", sr.ResourceTypeID)
	assert.Equal(t, entity.MeetingModeOffline, *sr.MeetingMode)
	assert.Equal(t, 12.9716, *sr.Latitude)
	assert.Equal(t, 77.5946, *sr.Longitude)
	assert.Equal(t, 10.0, *sr.RadiusKM)
	assert.Equal(t, start, sr.WindowStart.Unix())
	assert.Equal(t, end, sr.WindowEnd.Unix())
}

func TestSearchRequestFromProto_OptionalFieldsNil(t *testing.T) {
	req := &pb.SearchResourcesRequest{ResourceTypeId: "type-1"}

	sr := SearchRequestFromProto(req)

	assert.Equal(t, "type-1", sr.ResourceTypeID)
	assert.Nil(t, sr.TenantType)
	assert.Nil(t, sr.OrgID)
	assert.Nil(t, sr.Name)
	assert.Nil(t, sr.MeetingMode)
	assert.Nil(t, sr.Latitude)
	assert.Nil(t, sr.Longitude)
	assert.Nil(t, sr.RadiusKM)
	assert.Nil(t, sr.WindowStart)
	assert.Nil(t, sr.WindowEnd)
}

func TestSlotExceptionFromProto(t *testing.T) {
	se := SlotExceptionFromProto(&pb.AddSlotExceptionRequest{
		ResourceId: "res-1",
		StartUnix:  1000,
		EndUnix:    2000,
		Reason:     "extra coverage",
	})

	assert.Equal(t, "res-1", se.ResourceID)
	assert.Equal(t, int64(1000), se.Start.Unix())
	assert.Equal(t, int64(2000), se.End.Unix())
	assert.Equal(t, "extra coverage", se.Reason)
}

func TestLeavePeriodFromProto(t *testing.T) {
	lp := LeavePeriodFromProto(&pb.SetLeavePeriodRequest{
		ResourceId:   "res-1",
		StartDayUnix: 1000,
		EndDayUnix:   2000,
		Reason:       "vacation",
	})

	assert.Equal(t, "res-1", lp.ResourceID)
	assert.Equal(t, int64(1000), lp.Start.Unix())
	assert.Equal(t, int64(2000), lp.End.Unix())
	assert.Equal(t, "vacation", lp.Reason)
}

func TestSearchResponseToProto(t *testing.T) {
	resp := SearchResponseToProto([]entity.ResourceSummary{{
		ResourceID: "res-1",
		TenantType: entity.TenantTypeOrg,
		OrgID:      ToStringPtr("org-1"),
		Name:       "Dr. Rakesh",
		ResourceType: entity.ResourceType{
			ID:   "type-1",
			Name: "doctor",
		},
		MeetingMode: entity.MeetingModeOffline,
		DistanceKM:  ToFloatPtr(2.5),
		IsActive:    true,
		NextAvailableSlotTime: entity.SlotTiming{
			Start: time.Unix(1000, 0),
			End:   time.Unix(2000, 0),
		},
	}})

	got := resp.GetResources()[0]

	assert.Equal(t, "res-1", got.GetResourceId())
	assert.Equal(t, pb.TenantType_TENANT_TYPE_ORG, got.GetTenantType())
	assert.Equal(t, "org-1", got.GetOrgId())
	assert.Equal(t, "Dr. Rakesh", got.GetName())
	assert.Equal(t, "type-1", got.GetResourceType().GetResourceTypeId())
	assert.Equal(t, "doctor", got.GetResourceType().GetName())
	assert.Equal(t, pb.MeetingMode_MEETING_MODE_OFFLINE, got.GetMeetingMode())
	assert.Equal(t, 2.5, got.GetDistanceKm())
	assert.True(t, got.GetIsActive())
	assert.Equal(t, int64(1000), got.GetNextAvailableSlotTime().GetStartUnix())
	assert.Equal(t, int64(2000), got.GetNextAvailableSlotTime().GetEndUnix())
}

func TestSearchResponseToProto_OptionalFieldsNil(t *testing.T) {
	resp := SearchResponseToProto([]entity.ResourceSummary{{
		ResourceID: "res-1",
		TenantType: entity.TenantTypeIndividual,
		Name:       "Backend Panel",
		ResourceType: entity.ResourceType{
			ID:   "type-1",
			Name: "panel",
		},
		MeetingMode: entity.MeetingModeOnline,
		IsActive:    true,
	}})

	got := resp.GetResources()[0]

	assert.Equal(t, "res-1", got.GetResourceId())
	assert.Empty(t, got.GetOrgId())
	assert.Equal(t, 0.0, got.GetDistanceKm())
	assert.Equal(t, int64(0), got.GetNextAvailableSlotTime().GetStartUnix())
	assert.Equal(t, int64(0), got.GetNextAvailableSlotTime().GetEndUnix())
}

func TestSlotToProto(t *testing.T) {
	got := SlotToProto(entity.Slot{
		ID:         "slot-1",
		ResourceID: "res-1",
		SlotTiming: entity.SlotTiming{Start: time.Unix(1000, 0), End: time.Unix(2000, 0)},
		Status:     entity.SlotStatusOpen,
	})

	slot := got.GetSlot()

	assert.Equal(t, "slot-1", slot.GetSlotId())
	assert.Equal(t, "res-1", slot.GetResourceId())
	assert.Equal(t, pb.SlotStatus_SLOT_STATUS_OPEN, slot.GetStatus())
	assert.Equal(t, int64(1000), slot.GetSlotTiming().GetStartUnix())
	assert.Equal(t, int64(2000), slot.GetSlotTiming().GetEndUnix())
}

func TestResponseWrappers(t *testing.T) {
	assert.Equal(t, "res-1", CreateResourceResponseToProto("res-1").GetResourceId())
	assert.Equal(t, "res-1", UpdateResourceResponseToProto("res-1").GetResourceId())
	assert.Equal(t, int32(5), SetRecurringAvailabilityResponseToProto(5).GetSlotsGenerated())
	assert.Equal(t, "slot-1", AddSlotExceptionResponseToProto("slot-1", entity.SlotStatusOpen).GetSlotId())
	assert.Equal(t, pb.SlotStatus_SLOT_STATUS_BLOCKED, RemoveSlotExceptionResponseToProto(entity.SlotStatusBlocked).GetStatus())
	assert.Equal(t, int32(3), SetLeavePeriodResponseToProto(3).GetSlotsRemoved())
}

func TestValidateResourceSearchRequest_Valid(t *testing.T) {
	lat := 12.9716
	lng := 77.5946
	radius := 10.0

	req := entity.SearchResourceRequest{
		TenantType:     entity.TenantTypeOrg.ToPtr(),
		OrgID:          ToStringPtr("org-1"),
		Name:           ToStringPtr("doctor"),
		ResourceTypeID: "type-1",
		MeetingMode:    entity.MeetingModeOffline.ToPtr(),
		Latitude:       &lat,
		Longitude:      &lng,
		RadiusKM:       &radius,
	}

	assert.NoError(t, ValidateResourceSearchRequest(req))
}

func TestValidateResourceSearchRequest_MissingResourceType(t *testing.T) {
	assert.ErrorIs(t, ValidateResourceSearchRequest(entity.SearchResourceRequest{}), entity.ErrInvalidResourceTypeID)
}

func TestValidateResourceSearchRequest_InvalidMeetingMode(t *testing.T) {
	mode := entity.MeetingModeUnspecified

	req := entity.SearchResourceRequest{ResourceTypeID: "type-1", MeetingMode: &mode}

	assert.ErrorIs(t, ValidateResourceSearchRequest(req), entity.ErrInvalidMeetingMode)
}

func TestValidateResourceSearchRequest_InvalidLocation(t *testing.T) {
	tests := []struct {
		name string
		req  entity.SearchResourceRequest
	}{
		{
			name: "missing longitude",
			req: entity.SearchResourceRequest{
				ResourceTypeID: "type-1",
				Latitude:       ToFloatPtr(12.9716),
			},
		},
		{
			name: "invalid latitude",
			req: entity.SearchResourceRequest{
				ResourceTypeID: "type-1",
				Latitude:       ToFloatPtr(100),
				Longitude:      ToFloatPtr(77.5946),
				RadiusKM:       ToFloatPtr(10),
			},
		},
		{
			name: "invalid longitude",
			req: entity.SearchResourceRequest{
				ResourceTypeID: "type-1",
				Latitude:       ToFloatPtr(12.9716),
				Longitude:      ToFloatPtr(200),
				RadiusKM:       ToFloatPtr(10),
			},
		},
		{
			name: "invalid radius",
			req: entity.SearchResourceRequest{
				ResourceTypeID: "type-1",
				Latitude:       ToFloatPtr(12.9716),
				Longitude:      ToFloatPtr(77.5946),
				RadiusKM:       ToFloatPtr(0),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.ErrorIs(t, ValidateResourceSearchRequest(tt.req), entity.ErrUserLocationNotFound)
		})
	}
}

func TestValidateResourceSearchRequest_InvalidWindow(t *testing.T) {
	start := time.Date(2026, 8, 10, 0, 0, 0, 0, time.UTC)
	end := time.Date(2026, 8, 9, 0, 0, 0, 0, time.UTC)

	req := entity.SearchResourceRequest{
		ResourceTypeID: "type-1",
		WindowStart:    &start,
		WindowEnd:      &end,
	}

	assert.ErrorIs(t, ValidateResourceSearchRequest(req), entity.ErrInvalidTimeRange)
}

func TestValidateResourceSearchRequest_PartialWindow(t *testing.T) {
	start := time.Now()

	req := entity.SearchResourceRequest{ResourceTypeID: "type-1", WindowStart: &start}

	assert.ErrorIs(t, ValidateResourceSearchRequest(req), entity.ErrInvalidTimeRange)
}

func TestValidateResourceSearchRequest_NoOptionalFilters(t *testing.T) {
	req := entity.SearchResourceRequest{ResourceTypeID: "type-1"}

	assert.NoError(t, ValidateResourceSearchRequest(req))
}

func TestGetResourceByIdResponseToProto(t *testing.T) {
	summary := entity.ResourceSummary{
		ResourceID:  "res-1",
		TenantType:  entity.TenantTypeOrg,
		OrgID:       ToStringPtr("org-1"),
		Name:        "Dr. Rakesh",
		MeetingMode: entity.MeetingModeOffline,
		IsActive:    true,
	}

	attributes := map[string]string{
		"address":    "MG Road, Bangalore",
		"department": "ENT",
	}

	got := GetResourceByIdResponseToProto(summary, attributes)

	assert.Equal(t, "res-1", got.GetResource().GetResourceId())
	assert.Equal(t, "Dr. Rakesh", got.GetResource().GetName())
	assert.Equal(t, pb.TenantType_TENANT_TYPE_ORG, got.GetResource().GetTenantType())
	assert.Equal(t, "org-1", got.GetResource().GetOrgId())
	assert.Equal(t, pb.MeetingMode_MEETING_MODE_OFFLINE, got.GetResource().GetMeetingMode())
	assert.True(t, got.GetResource().GetIsActive())
	assert.Equal(t, attributes, got.GetAttributes())
	assert.Nil(t, got.GetResource().GetNextAvailableSlotTime())
}

func TestGetSlotsByResourceIdResponseToProto(t *testing.T) {
	slots := []entity.Slot{
		{
			ID:         "slot-1",
			ResourceID: "res-1",
			SlotTiming: entity.SlotTiming{
				Start: time.Unix(1000, 0),
				End:   time.Unix(1100, 0),
			},
			Status: entity.SlotStatusOpen,
		},
		{
			ID:         "slot-2",
			ResourceID: "res-1",
			SlotTiming: entity.SlotTiming{
				Start: time.Unix(1200, 0),
				End:   time.Unix(1300, 0),
			},
			Status: entity.SlotStatusBooked,
		},
	}

	recurrence := []entity.RecurrenceRule{
		{
			Day:      entity.Monday,
			Timezone: "Asia/Kolkata",
			Slots: []entity.TimeSlot{
				{
					StartHour:   10,
					StartMinute: 0,
					EndHour:     11,
					EndMinute:   0,
				},
			},
		},
	}

	got := GetSlotsByResourceIdResponseToProto(recurrence, slots)

	assert.Len(t, got.GetSlots(), 2)
	assert.Equal(t, "slot-1", got.GetSlots()[0].GetSlotId())
	assert.Equal(t, "res-1", got.GetSlots()[0].GetResourceId())
	assert.Equal(t, int64(1000), got.GetSlots()[0].GetSlotTiming().GetStartUnix())
	assert.Equal(t, int64(1100), got.GetSlots()[0].GetSlotTiming().GetEndUnix())
	assert.Equal(t, pb.SlotStatus_SLOT_STATUS_OPEN, got.GetSlots()[0].GetStatus())

	assert.Equal(t, "slot-2", got.GetSlots()[1].GetSlotId())
	assert.Equal(t, pb.SlotStatus_SLOT_STATUS_BOOKED, got.GetSlots()[1].GetStatus())

	assert.Len(t, got.GetRecurrence(), 1)
	assert.Equal(t, pb.DayOfWeek_DAY_OF_WEEK_MONDAY, got.GetRecurrence()[0].GetDay())
	assert.Equal(t, "Asia/Kolkata", got.GetRecurrence()[0].GetTimezone())
	assert.Len(t, got.GetRecurrence()[0].GetSlots(), 1)
	assert.Equal(t, int32(10), got.GetRecurrence()[0].GetSlots()[0].GetStartHour())
	assert.Equal(t, int32(0), got.GetRecurrence()[0].GetSlots()[0].GetStartMinute())
	assert.Equal(t, int32(11), got.GetRecurrence()[0].GetSlots()[0].GetEndHour())
	assert.Equal(t, int32(0), got.GetRecurrence()[0].GetSlots()[0].GetEndMinute())
}

func TestGetSlotsByResourceIdResponseToProto_Empty(t *testing.T) {
	got := GetSlotsByResourceIdResponseToProto(nil, nil)

	assert.Empty(t, got.GetSlots())
	assert.Empty(t, got.GetRecurrence())
}
