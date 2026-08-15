package util

import (
	"time"

	"github.com/PriyanshuTrivedi/nexus-scheduler/code/resource/entity"
	pb "github.com/PriyanshuTrivedi/nexus-scheduler/gen/idl/resource"
)

func meetingModeFromProto(v pb.MeetingMode) entity.MeetingMode { return entity.MeetingMode(v) }
func meetingModeToProto(v entity.MeetingMode) pb.MeetingMode   { return pb.MeetingMode(v) }
func dayFromProto(v pb.DayOfWeek) entity.DayOfWeek             { return entity.DayOfWeek(v) }
func dayToProto(v entity.DayOfWeek) pb.DayOfWeek               { return pb.DayOfWeek(v) }
func slotStatusFromProto(v pb.SlotStatus) entity.SlotStatus    { return entity.SlotStatus(v) }
func slotStatusToProto(v entity.SlotStatus) pb.SlotStatus      { return pb.SlotStatus(v) }

func RecurrenceRulesFromProto(in []*pb.RecurrenceRule) []entity.RecurrenceRule {
	out := make([]entity.RecurrenceRule, 0, len(in))
	for _, r := range in {
		slots := make([]entity.TimeSlot, 0, len(r.Slots))
		for _, s := range r.Slots {
			slots = append(slots, entity.TimeSlot{
				StartHour: int(s.StartHour), StartMinute: int(s.StartMinute),
				EndHour: int(s.EndHour), EndMinute: int(s.EndMinute),
			})
		}
		out = append(out, entity.RecurrenceRule{Day: dayFromProto(r.Day), Timezone: r.Timezone, Slots: slots})
	}
	return out
}

func recurrenceToProto(in []entity.RecurrenceRule) []*pb.RecurrenceRule {
	out := make([]*pb.RecurrenceRule, 0, len(in))
	for _, r := range in {
		slots := make([]*pb.TimeSlot, 0, len(r.Slots))
		for _, s := range r.Slots {
			slots = append(slots, &pb.TimeSlot{StartHour: int32(s.StartHour), StartMinute: int32(s.StartMinute), EndHour: int32(s.EndHour), EndMinute: int32(s.EndMinute)})
		}
		out = append(out, &pb.RecurrenceRule{Day: dayToProto(r.Day), Timezone: r.Timezone, Slots: slots})
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
	var lat, lng *float64
	if req.Lat != nil {
		v := req.GetLat()
		lat = &v
	}
	if req.Lng != nil {
		v := req.GetLng()
		lng = &v
	}
	return entity.Resource{
		TenantType: tenantTypeFromProto(req.TenantType),
		OrgID:      req.OrgId, UserID: req.UserId, ResourceTypeID: req.ResourceTypeId, Name: req.Name,
		MeetingMode: meetingModeFromProto(req.MeetingMode), Latitude: lat, Longitude: lng,
		Attributes: req.Attributes, Recurrence: RecurrenceRulesFromProto(req.Recurrence), IsActive: true,
	}
}

func SlotExceptionFromProto(req *pb.AddSlotExceptionRequest) entity.SlotException {
	return entity.SlotException{ResourceID: req.ResourceId, Start: time.Unix(req.StartUnix, 0), End: time.Unix(req.EndUnix, 0), Reason: req.Reason}
}

func LeavePeriodFromProto(req *pb.SetLeavePeriodRequest) entity.LeavePeriod {
	return entity.LeavePeriod{ResourceID: req.ResourceId, Start: time.Unix(req.StartDayUnix, 0), End: time.Unix(req.EndDayUnix, 0), Reason: req.Reason}
}

func SearchRequestFromProto(req *pb.SearchResourcesRequest) entity.SearchResourceRequest {
	sr := entity.SearchResourceRequest{
		TenantType: tenantTypeFromProto(req.TenantType),
		OrgID:      req.OrgId, Name: req.Name, ResourceTypeID: req.ResourceTypeId,
		MeetingMode: meetingModeFromProto(req.MeetingMode), Attributes: req.Attributes,
		Latitude: req.Lat, Longitude: req.Lng, RadiusKM: req.RadiusKm,
	}
	if req.WindowStartUnix != 0 {
		sr.WindowStart = time.Unix(req.WindowStartUnix, 0)
	}
	if req.WindowEndUnix != 0 {
		sr.WindowEnd = time.Unix(req.WindowEndUnix, 0)
	}
	return sr
}

func SearchResponseToProto(resources []entity.ResourceSummary) *pb.SearchResourcesResponse {
	resp := &pb.SearchResourcesResponse{}
	for _, r := range resources {
		slots := make([]*pb.Slot, 0, len(r.NextAvailableSlots))
		for _, s := range r.NextAvailableSlots {
			slots = append(slots, &pb.Slot{StartUnix: s.Start.Unix(), EndUnix: s.End.Unix()})
		}
		attrs := make(map[string]string, len(r.Attributes))
		for k, v := range r.Attributes {
			if k != "__user_id" {
				attrs[k] = v
			}
		}
		item := &pb.ResourceSummary{
			ResourceId: r.ResourceID, TenantType: tenantTypeToProto(r.TenantType), Name: r.Name,
			ResourceType: &pb.ResourceType{ResourceTypeId: r.ResourceType.ID, Name: r.ResourceType.Name, IsActive: r.ResourceType.IsActive},
			MeetingMode:  meetingModeToProto(r.MeetingMode), DistanceKm: r.DistanceKM, Attributes: attrs,
			NextAvailableSlots: slots, IsActive: r.IsActive,
		}
		if r.OrgID != "" {
			item.OrgId = &r.OrgID
		}
		resp.Resources = append(resp.Resources, item)
	}
	return resp
}

func ResourceTypeToProto(rt entity.ResourceType) *pb.ResourceType {
	return &pb.ResourceType{ResourceTypeId: rt.ID, Name: rt.Name, IsActive: rt.IsActive}
}

func CreateResourceTypeResponseToProto(rt entity.ResourceType) *pb.CreateResourceTypeResponse {
	return &pb.CreateResourceTypeResponse{ResourceType: ResourceTypeToProto(rt)}
}

func SlotToProto(slot entity.Slot) *pb.GetSlotResponse {
	return &pb.GetSlotResponse{SlotId: slot.ID, ResourceId: slot.ResourceID, StartUnix: slot.Start.Unix(), EndUnix: slot.End.Unix(), Status: slotStatusToProto(slot.Status)}
}

func CreateResourceResponseToProto(id string) *pb.CreateResourceResponse {
	return &pb.CreateResourceResponse{ResourceId: id}
}
func SetRecurringAvailabilityResponseToProto(n int) *pb.SetRecurringAvailabilityResponse {
	return &pb.SetRecurringAvailabilityResponse{SlotsGenerated: int32(n)}
}
func AddSlotExceptionResponseToProto(slotID string, status entity.SlotStatus) *pb.AddSlotExceptionResponse {
	return &pb.AddSlotExceptionResponse{SlotId: slotID, Status: slotStatusToProto(status)}
}
func RemoveSlotExceptionResponseToProto(status entity.SlotStatus) *pb.RemoveSlotExceptionResponse {
	return &pb.RemoveSlotExceptionResponse{Status: slotStatusToProto(status)}
}
func SetLeavePeriodResponseToProto(n int) *pb.SetLeavePeriodResponse {
	return &pb.SetLeavePeriodResponse{SlotsRemoved: int32(n)}
}
