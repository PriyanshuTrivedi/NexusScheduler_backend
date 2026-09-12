package util

import (
	"time"

	"github.com/PriyanshuTrivedi/nexus-scheduler/code/resource/entity"
	pb "github.com/PriyanshuTrivedi/nexus-scheduler/gen/idl/resource"
)

func meetingModeFromProto(v pb.MeetingMode) entity.MeetingMode {
	return entity.MeetingMode(v)
}

func meetingModeToProto(v entity.MeetingMode) pb.MeetingMode {
	return pb.MeetingMode(v)
}

func dayFromProto(v pb.DayOfWeek) entity.DayOfWeek {
	return entity.DayOfWeek(v)
}

func dayToProto(v entity.DayOfWeek) pb.DayOfWeek {
	return pb.DayOfWeek(v)
}

func slotStatusToProto(v entity.SlotStatus) pb.SlotStatus {
	return pb.SlotStatus(v)
}

func RecurrenceRulesFromProto(in []*pb.RecurrenceRule) []entity.RecurrenceRule {
	out := make([]entity.RecurrenceRule, 0, len(in))
	for _, r := range in {
		slots := make([]entity.TimeSlot, 0, len(r.Slots))
		for _, s := range r.Slots {
			slots = append(slots, entity.TimeSlot{
				StartHour:   int(s.StartHour),
				StartMinute: int(s.StartMinute),
				EndHour:     int(s.EndHour),
				EndMinute:   int(s.EndMinute),
			})
		}

		out = append(out, entity.RecurrenceRule{
			Day:      dayFromProto(r.Day),
			Timezone: r.Timezone,
			Slots:    slots,
		})
	}
	return out
}

func recurrenceToProto(in []entity.RecurrenceRule) []*pb.RecurrenceRule {
	out := make([]*pb.RecurrenceRule, 0, len(in))
	for _, r := range in {
		slots := make([]*pb.TimeSlot, 0, len(r.Slots))
		for _, s := range r.Slots {
			slots = append(slots, &pb.TimeSlot{
				StartHour:   int32(s.StartHour),
				StartMinute: int32(s.StartMinute),
				EndHour:     int32(s.EndHour),
				EndMinute:   int32(s.EndMinute),
			})
		}

		out = append(out, &pb.RecurrenceRule{
			Day:      dayToProto(r.Day),
			Timezone: r.Timezone,
			Slots:    slots,
		})
	}

	return out
}

func tenantTypeFromProto(t pb.TenantType) entity.TenantType {
	switch t {
	case pb.TenantType_TENANT_TYPE_INDIVIDUAL:
		return entity.TenantTypeIndividual
	case pb.TenantType_TENANT_TYPE_ORG:
		return entity.TenantTypeOrg
	default:
		return entity.TenantTypeUnspecified
	}
}

func tenantTypeToProto(t entity.TenantType) pb.TenantType {
	switch t {
	case entity.TenantTypeIndividual:
		return pb.TenantType_TENANT_TYPE_INDIVIDUAL
	case entity.TenantTypeOrg:
		return pb.TenantType_TENANT_TYPE_ORG
	default:
		return pb.TenantType_TENANT_TYPE_UNSPECIFIED
	}
}

func ResourceFromCreateRequest(req *pb.CreateResourceRequest) entity.Resource {
	r := entity.Resource{
		UserID:     req.GetUserId(),
		TenantType: tenantTypeFromProto(req.GetTenantType()),
		ResourceType: entity.ResourceType{
			ID: req.GetResourceTypeId(),
		},
		Name:        req.GetName(),
		MeetingMode: meetingModeFromProto(req.GetMeetingMode()),
		Attributes:  req.Attributes,
		Recurrence:  RecurrenceRulesFromProto(req.GetRecurrence()),
		IsActive:    true,
	}
	if req.Address != nil {
		r.Address = ToStringPtr(req.GetAddress())
	}
	if req.OrgId != nil {
		r.OrgID = ToStringPtr(req.GetOrgId())
	}
	return r
}

func ResourceFromUpdateRequest(req *pb.UpdateResourceRequest) entity.Resource {
	r := entity.Resource{
		ID:          req.GetResourceId(),
		TenantType:  tenantTypeFromProto(req.GetTenantType()),
		Name:        req.Name,
		MeetingMode: meetingModeFromProto(req.GetMeetingMode()),
		Attributes:  req.Attributes,
	}
	if req.Address != nil {
		r.Address = ToStringPtr(req.GetAddress())
	}
	if req.OrgId != nil {
		r.OrgID = ToStringPtr(req.GetOrgId())
	}
	return r
}

func SlotExceptionFromProto(req *pb.AddSlotExceptionRequest) entity.SlotException {
	return entity.SlotException{
		ResourceID: req.ResourceId,
		Start:      time.Unix(req.StartUnix, 0),
		End:        time.Unix(req.EndUnix, 0),
		Reason:     req.Reason,
	}
}

func LeavePeriodFromProto(req *pb.SetLeavePeriodRequest) entity.LeavePeriod {
	return entity.LeavePeriod{
		ResourceID: req.ResourceId,
		Start:      time.Unix(req.StartDayUnix, 0),
		End:        time.Unix(req.EndDayUnix, 0),
		Reason:     req.Reason,
	}
}

func SearchRequestFromProto(req *pb.SearchResourcesRequest) entity.SearchResourceRequest {
	sr := entity.SearchResourceRequest{
		ResourceTypeID: req.GetResourceTypeId(),
	}
	if req.Attributes != nil {
		sr.Attributes = req.Attributes
	}
	if req.MeetingMode != nil {
		sr.MeetingMode = meetingModeFromProto(req.GetMeetingMode()).ToPtr()
	}
	if req.Name != nil {
		sr.Name = ToStringPtr(req.GetName())
	}
	if req.TenantType != nil {
		tenantType := tenantTypeFromProto(req.GetTenantType())
		sr.TenantType = &tenantType
	}
	if req.OrgId != nil {
		sr.OrgID = ToStringPtr(req.GetOrgId())
	}
	if req.Lat != nil {
		sr.Latitude = ToFloatPtr(req.GetLat())
	}
	if req.Lng != nil {
		sr.Longitude = ToFloatPtr(req.GetLng())
	}
	if req.RadiusKm != nil {
		sr.RadiusKM = ToFloatPtr(req.GetRadiusKm())
	}
	if req.WindowStartUnix != nil {
		sr.WindowStart = ToTimePtr(time.Unix(*req.WindowStartUnix, 0))
	}
	if req.WindowEndUnix != nil {
		sr.WindowEnd = ToTimePtr(time.Unix(*req.WindowEndUnix, 0))
	}
	return sr
}

func ResourceSummaryToProto(r entity.ResourceSummary) *pb.ResourceSummary {
	item := &pb.ResourceSummary{
		ResourceId:            r.ResourceID,
		TenantType:            tenantTypeToProto(r.TenantType),
		Name:                  r.Name,
		ResourceType:          ResourceTypeToProto(r.ResourceType),
		MeetingMode:           meetingModeToProto(r.MeetingMode),
		IsActive:              r.IsActive,
		NextAvailableSlotTime: SlotTimingToProto(r.NextAvailableSlotTime),
	}
	if r.DistanceKM != nil {
		item.DistanceKm = ToFloatPtr(*r.DistanceKM)
	}
	if r.OrgID != nil {
		item.OrgId = ToStringPtr(*r.OrgID)
	}
	return item
}

func SearchResponseToProto(resources []entity.ResourceSummary) *pb.SearchResourcesResponse {
	resp := &pb.SearchResourcesResponse{
		Resources: make([]*pb.ResourceSummary, 0, len(resources)),
	}
	for _, r := range resources {
		resp.Resources = append(resp.Resources, ResourceSummaryToProto(r))
	}
	return resp
}

func SlotTimingToProto(slot entity.SlotTiming) *pb.SlotTiming {
	if slot.Start.IsZero() || slot.End.IsZero() {
		return nil
	}
	return &pb.SlotTiming{
		StartUnix: slot.Start.Unix(),
		EndUnix:   slot.End.Unix(),
	}
}

func SlotToProto(slot entity.Slot) *pb.GetSlotResponse {
	return &pb.GetSlotResponse{
		Slot: &pb.Slot{
			SlotId:     slot.ID,
			ResourceId: slot.ResourceID,
			SlotTiming: SlotTimingToProto(slot.SlotTiming),
			Status:     slotStatusToProto(slot.Status),
		},
	}
}

func GetResourceByIdResponseToProto(summary entity.ResourceSummary, attributes map[string]string) *pb.GetResourceByIdResponse {
	return &pb.GetResourceByIdResponse{
		Resource:   ResourceSummaryToProto(summary),
		Attributes: attributes,
	}
}

func GetSlotsByResourceIdResponseToProto(recurrence []entity.RecurrenceRule, slots []entity.Slot) *pb.GetSlotsByResourceIdResponse {
	resp := &pb.GetSlotsByResourceIdResponse{
		Recurrence: recurrenceToProto(recurrence),
		Slots:      make([]*pb.Slot, 0, len(slots)),
	}
	for _, slot := range slots {
		resp.Slots = append(resp.Slots, &pb.Slot{
			SlotId:     slot.ID,
			ResourceId: slot.ResourceID,
			SlotTiming: SlotTimingToProto(slot.SlotTiming),
			Status:     slotStatusToProto(slot.Status),
		})
	}
	return resp
}

func ResourceTypeToProto(rt entity.ResourceType) *pb.ResourceType {
	return &pb.ResourceType{
		ResourceTypeId: rt.ID,
		Name:           rt.Name,
		IsActive:       rt.IsActive,
	}
}

func CreateResourceTypeResponseToProto(rt entity.ResourceType) *pb.CreateResourceTypeResponse {
	return &pb.CreateResourceTypeResponse{
		ResourceType: ResourceTypeToProto(rt),
	}
}

func CreateResourceResponseToProto(id string) *pb.CreateResourceResponse {
	return &pb.CreateResourceResponse{
		ResourceId: id,
	}
}

func UpdateResourceResponseToProto(id string) *pb.UpdateResourceResponse {
	return &pb.UpdateResourceResponse{
		ResourceId: id,
	}
}

func SetRecurringAvailabilityResponseToProto(n int) *pb.SetRecurringAvailabilityResponse {
	return &pb.SetRecurringAvailabilityResponse{
		SlotsGenerated: int32(n),
	}
}

func AddSlotExceptionResponseToProto(slotID string, status entity.SlotStatus) *pb.AddSlotExceptionResponse {
	return &pb.AddSlotExceptionResponse{
		SlotId: slotID,
		Status: slotStatusToProto(status),
	}
}

func RemoveSlotExceptionResponseToProto(status entity.SlotStatus) *pb.RemoveSlotExceptionResponse {
	return &pb.RemoveSlotExceptionResponse{
		Status: slotStatusToProto(status),
	}
}

func SetLeavePeriodResponseToProto(n int) *pb.SetLeavePeriodResponse {
	return &pb.SetLeavePeriodResponse{
		SlotsRemoved: int32(n),
	}
}

func ValidateResourceCreate(r entity.Resource) error {
	if r.TenantType == entity.TenantTypeUnspecified {
		return entity.ErrInvalidTenantType
	}
	if r.TenantType == entity.TenantTypeIndividual && r.OrgID != nil {
		return entity.ErrInvalidOrganizationID
	}
	if r.TenantType == entity.TenantTypeOrg && r.OrgID == nil {
		return entity.ErrInvalidOrganizationID
	}
	if r.ResourceType.ID == "" {
		return entity.ErrInvalidResourceTypeID
	}
	if r.Name == "" {
		return entity.ErrInvalidName
	}
	if !r.MeetingMode.Valid() {
		return entity.ErrInvalidMeetingMode
	}
	if r.MeetingMode.RequiresLocation() && r.Address == nil {
		return entity.ErrLocationRequired
	}
	for _, rule := range r.Recurrence {
		if err := rule.Validate(); err != nil {
			return err
		}
	}
	return nil
}

func ValidateResourceUpdate(r entity.Resource) error {
	if r.ID == "" {
		return entity.ErrInvalidResourceID
	}
	if r.Name == "" {
		return entity.ErrInvalidName
	}
	if r.TenantType == entity.TenantTypeIndividual && r.OrgID != nil {
		return entity.ErrInvalidOrganizationID
	}
	if r.TenantType == entity.TenantTypeOrg && r.OrgID == nil {
		return entity.ErrInvalidOrganizationID
	}
	if !r.MeetingMode.Valid() {
		return entity.ErrInvalidMeetingMode
	}
	if r.MeetingMode.RequiresLocation() && r.Address == nil {
		return entity.ErrLocationRequired
	}
	return nil
}

func ValidateResourceSearchRequest(r entity.SearchResourceRequest) error {
	// Internal resource-account lookups use __user_id instead of a public
	// resource-type filter. Public searches still require resource_type_id.
	if r.ResourceTypeID == "" {
		if _, ok := r.Attributes["__user_id"]; !ok {
			return entity.ErrInvalidResourceTypeID
		}
	}

	if r.MeetingMode != nil && !r.MeetingMode.Valid() {
		return entity.ErrInvalidMeetingMode
	}

	if (r.Latitude == nil) != (r.Longitude == nil) {
		return entity.ErrUserLocationNotFound
	}

	if r.Latitude != nil || r.Longitude != nil || r.RadiusKM != nil {
		if r.Latitude == nil || r.Longitude == nil || r.RadiusKM == nil {
			return entity.ErrUserLocationNotFound
		}
		if *r.Latitude < -90 || *r.Latitude > 90 {
			return entity.ErrUserLocationNotFound
		}
		if *r.Longitude < -180 || *r.Longitude > 180 {
			return entity.ErrUserLocationNotFound
		}
		if *r.RadiusKM <= 0 {
			return entity.ErrUserLocationNotFound
		}
	}
	if (r.WindowStart == nil) != (r.WindowEnd == nil) {
		return entity.ErrInvalidTimeRange
	}
	if r.WindowStart != nil && !r.WindowEnd.After(*r.WindowStart) {
		return entity.ErrInvalidTimeRange
	}

	return nil
}

func ToStringPtr(s string) *string {
	return &s
}
func ToFloatPtr(f float64) *float64 {
	return &f
}
func ToTimePtr(t time.Time) *time.Time {
	return &t
}
