package handler

import (
	"context"
	"errors"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	"github.com/PriyanshuTrivedi/nexus-scheduler/code/resource/controller"
	"github.com/PriyanshuTrivedi/nexus-scheduler/code/resource/entity"
	"github.com/PriyanshuTrivedi/nexus-scheduler/code/resource/store"
	"github.com/PriyanshuTrivedi/nexus-scheduler/code/resource/util"
	pb "github.com/PriyanshuTrivedi/nexus-scheduler/gen/idl/resource"
)

type Handler struct {
	pb.UnimplementedResourceServiceServer
	controller controller.Controller
}

func New(c controller.Controller) *Handler {
	return &Handler{controller: c}
}

func (h *Handler) ListResourceTypes(ctx context.Context, _ *pb.ListResourceTypesRequest) (*pb.ListResourceTypesResponse, error) {
	types, err := h.controller.ListResourceTypes(ctx)
	if err != nil {
		return nil, mapErr(err)
	}
	resp := &pb.ListResourceTypesResponse{ResourceTypes: make([]*pb.ResourceType, 0, len(types))}
	for _, rt := range types {
		resp.ResourceTypes = append(resp.ResourceTypes, util.ResourceTypeToProto(rt))
	}
	return resp, nil
}

func (h *Handler) CreateResourceType(ctx context.Context, req *pb.CreateResourceTypeRequest) (*pb.CreateResourceTypeResponse, error) {
	rt, err := h.controller.CreateResourceType(ctx, req.Name)
	if err != nil {
		return nil, mapErr(err)
	}
	return util.CreateResourceTypeResponseToProto(rt), nil
}

func (h *Handler) DeleteResourceType(ctx context.Context, req *pb.DeleteResourceTypeRequest) (*pb.DeleteResourceTypeResponse, error) {
	if err := h.controller.DeleteResourceType(ctx, req.ResourceTypeId); err != nil {
		return nil, mapErr(err)
	}
	return &pb.DeleteResourceTypeResponse{}, nil
}

func (h *Handler) SetResourceTypeStatus(ctx context.Context, req *pb.SetResourceTypeStatusRequest) (*pb.SetResourceTypeStatusResponse, error) {
	rt, err := h.controller.SetResourceTypeStatus(ctx, req.ResourceTypeId, req.IsActive)
	if err != nil {
		return nil, mapErr(err)
	}
	return &pb.SetResourceTypeStatusResponse{ResourceType: util.ResourceTypeToProto(rt)}, nil
}

func (h *Handler) CreateResource(ctx context.Context, req *pb.CreateResourceRequest) (*pb.CreateResourceResponse, error) {
	if md, ok := metadata.FromIncomingContext(ctx); ok {
		if values := md.Get("x-resource-operation"); len(values) > 0 && values[0] == "update" {
			updater, ok := h.controller.(interface {
				UpdateResource(context.Context, string, string, entity.MeetingMode, *float64, *float64, map[string]string) (string, error)
			})
			if !ok {
				return nil, status.Error(codes.Internal, "resource profile update is unavailable")
			}
			id, err := updater.UpdateResource(
				ctx,
				req.GetUserId(),
				req.GetName(),
				meetingModeFromProto(req.GetMeetingMode()),
				req.Lat,
				req.Lng,
				req.GetAttributes(),
			)
			if err != nil {
				return nil, mapErr(err)
			}
			return util.CreateResourceResponseToProto(id), nil
		}
	}

	r := util.ResourceFromCreateRequest(req)
	id, err := h.controller.CreateResource(ctx, r)
	if err != nil {
		return nil, mapErr(err)
	}
	return util.CreateResourceResponseToProto(id), nil
}

func meetingModeFromProto(v pb.MeetingMode) entity.MeetingMode {
	switch v {
	case pb.MeetingMode_MEETING_MODE_ONLINE:
		return entity.MeetingModeOnline
	case pb.MeetingMode_MEETING_MODE_OFFLINE:
		return entity.MeetingModeOffline
	case pb.MeetingMode_MEETING_MODE_HYBRID:
		return entity.MeetingModeHybrid
	default:
		return entity.MeetingModeUnspecified
	}
}

func (h *Handler) SetResourceStatus(ctx context.Context, req *pb.SetResourceStatusRequest) (*pb.SetResourceStatusResponse, error) {
	if err := h.controller.SetResourceStatus(ctx, req.ResourceId, req.IsActive); err != nil {
		return nil, mapErr(err)
	}
	return &pb.SetResourceStatusResponse{IsActive: req.IsActive}, nil
}

func (h *Handler) DeleteResource(ctx context.Context, req *pb.DeleteResourceRequest) (*pb.DeleteResourceResponse, error) {
	if err := h.controller.DeleteResource(ctx, req.ResourceId); err != nil {
		return nil, mapErr(err)
	}
	return &pb.DeleteResourceResponse{}, nil
}

func (h *Handler) SetRecurringAvailability(ctx context.Context, req *pb.SetRecurringAvailabilityRequest) (*pb.SetRecurringAvailabilityResponse, error) {
	rules := util.RecurrenceRulesFromProto(req.Recurrence)
	n, err := h.controller.SetRecurringAvailability(ctx, req.ResourceId, rules)
	if err != nil {
		return nil, mapErr(err)
	}
	return util.SetRecurringAvailabilityResponseToProto(n), nil
}

func (h *Handler) AddSlotException(ctx context.Context, req *pb.AddSlotExceptionRequest) (*pb.AddSlotExceptionResponse, error) {
	se := util.SlotExceptionFromProto(req)
	slot, err := h.controller.AddSlotException(ctx, se)
	if err != nil {
		return nil, mapErr(err)
	}
	return util.AddSlotExceptionResponseToProto(slot.ID, slot.Status), nil
}

func (h *Handler) RemoveSlotException(ctx context.Context, req *pb.RemoveSlotExceptionRequest) (*pb.RemoveSlotExceptionResponse, error) {
	status_, err := h.controller.RemoveSlotException(ctx, req.SlotId, req.Reason)
	if err != nil {
		return nil, mapErr(err)
	}
	return util.RemoveSlotExceptionResponseToProto(status_), nil
}

func (h *Handler) SetLeavePeriod(ctx context.Context, req *pb.SetLeavePeriodRequest) (*pb.SetLeavePeriodResponse, error) {
	lp := util.LeavePeriodFromProto(req)
	n, err := h.controller.SetLeavePeriod(ctx, lp)
	if err != nil {
		return nil, mapErr(err)
	}
	return util.SetLeavePeriodResponseToProto(n), nil
}

func (h *Handler) SearchResources(ctx context.Context, req *pb.SearchResourcesRequest) (*pb.SearchResourcesResponse, error) {
	sr := util.SearchRequestFromProto(req)
	summaries, err := h.controller.SearchResources(ctx, sr)
	if err != nil {
		return nil, mapErr(err)
	}
	return util.SearchResponseToProto(summaries), nil
}

func (h *Handler) GetSlot(ctx context.Context, req *pb.GetSlotRequest) (*pb.GetSlotResponse, error) {
	slot, err := h.controller.GetSlot(ctx, req.SlotId)
	if err != nil {
		return nil, mapErr(err)
	}
	return util.SlotToProto(slot), nil
}

func mapErr(err error) error {
	switch {
	case errors.Is(err, store.ErrResourceNotFound),
		errors.Is(err, store.ErrResourceTypeNotFound),
		errors.Is(err, store.ErrSlotNotFound):
		return status.Error(codes.NotFound, err.Error())
	case errors.Is(err, store.ErrResourceUnavailable),
		errors.Is(err, store.ErrResourceTypeInUse),
		errors.Is(err, store.ErrResourceTypeMustBeInactive),
		errors.Is(err, store.ErrResourceMustBeInactive):
		return status.Error(codes.FailedPrecondition, err.Error())
	case errors.Is(err, store.ErrSlotAlreadyExists),
		errors.Is(err, store.ErrResourceTypeExists):
		return status.Error(codes.AlreadyExists, err.Error())
	case errors.Is(err, entity.ErrInvalidOrganizationID),
		errors.Is(err, entity.ErrInvalidUserID),
		errors.Is(err, entity.ErrInvalidTenantType),
		errors.Is(err, entity.ErrInvalidName),
		errors.Is(err, entity.ErrInvalidResourceID),
		errors.Is(err, entity.ErrInvalidResourceTypeID),
		errors.Is(err, entity.ErrInvalidResourceTypeName),
		errors.Is(err, entity.ErrInvalidSlot),
		errors.Is(err, entity.ErrInvalidTimeRange),
		errors.Is(err, entity.ErrInvalidMeetingMode),
		errors.Is(err, entity.ErrLocationRequired),
		errors.Is(err, entity.ErrInvalidDay),
		errors.Is(err, entity.ErrInvalidTimezone),
		errors.Is(err, entity.ErrEmptyRecurrenceRule),
		errors.Is(err, entity.ErrOverlappingSlots):
		return status.Error(codes.InvalidArgument, err.Error())
	default:
		return status.Error(codes.Internal, err.Error())
	}
}
