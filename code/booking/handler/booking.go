package handler

import (
	"context"

	"github.com/PriyanshuTrivedi/nexus-scheduler/code/booking/controller"
	"github.com/PriyanshuTrivedi/nexus-scheduler/code/booking/entity"
	"github.com/PriyanshuTrivedi/nexus-scheduler/code/booking/store"
	"github.com/PriyanshuTrivedi/nexus-scheduler/code/booking/util"

	bookingpb "github.com/PriyanshuTrivedi/nexus-scheduler/gen/idl/booking"
	"google.golang.org/grpc/metadata"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type Handler struct {
	bookingpb.UnimplementedBookingServiceServer
	ctl controller.Controller
}

func New(ctl controller.Controller) *Handler {
	return &Handler{ctl: ctl}
}

func (h *Handler) CreateBooking(ctx context.Context, req *bookingpb.CreateBookingRequest) (*bookingpb.CreateBookingResponse, error) {
	b := util.CreateRequestToBooking(req)
	created, err := h.ctl.CreateBooking(ctx, b)
	if err != nil {
		if isValidationErr(err) {
			return nil, status.Error(codes.InvalidArgument, err.Error())
		}
		if err == store.ErrSlotAlreadyBooked {
			return nil, status.Error(codes.AlreadyExists, err.Error())
		}
		return nil, status.Error(codes.Internal, err.Error())
	}
	return util.BookingToCreateResponse(created), nil
}

func (h *Handler) CancelBooking(ctx context.Context, req *bookingpb.CancelBookingRequest) (*bookingpb.CancelBookingResponse, error) {
	result, err := h.ctl.CancelBooking(ctx, req.GetReferenceCode())
	if err != nil {
		if err == store.ErrNotFound {
			return nil, status.Error(codes.NotFound, "booking not found")
		}
		return nil, status.Error(codes.Internal, err.Error())
	}
	return util.CancelResponse(result), nil
}

func (h *Handler) RescheduleBooking(ctx context.Context, req *bookingpb.RescheduleBookingRequest) (*bookingpb.RescheduleBookingResponse, error) {
	b, err := h.ctl.RescheduleBooking(ctx, req.GetReferenceCode(), req.GetStartUnix(), req.GetEndUnix())
	if err != nil {
		if err == store.ErrNotFound {
			return nil, status.Error(codes.NotFound, "booking not found")
		}
		if err == store.ErrSlotAlreadyBooked {
			return nil, status.Error(codes.AlreadyExists, "new slot already taken")
		}
		return nil, status.Error(codes.Internal, err.Error())
	}
	return util.BookingToRescheduleResponse(b), nil
}

func (h *Handler) ListUpcomingBookings(ctx context.Context, req *bookingpb.ListUserBookingsRequest) (*bookingpb.ListUserBookingsResponse, error) {
	if md, ok := metadata.FromIncomingContext(ctx); ok && len(md.Get("x-list-scope")) > 0 && md.Get("x-list-scope")[0] == "resource" {
		bookings, err := h.ctl.ListResourceBookings(ctx, req.GetUserId(), true)
		if err != nil {
			if err == entity.ErrInvalidResourceID {
				return nil, status.Error(codes.InvalidArgument, err.Error())
			}
			return nil, status.Error(codes.Internal, err.Error())
		}
		resp := &bookingpb.ListUserBookingsResponse{Bookings: make([]*bookingpb.GetBookingStatusResponse, 0, len(bookings))}
		for _, b := range bookings {
			resp.Bookings = append(resp.Bookings, util.BookingToStatusResponse(b))
		}
		return resp, nil
	}
	return h.listUserBookings(ctx, req.GetUserId(), true)
}

func (h *Handler) ListPastBookings(ctx context.Context, req *bookingpb.ListUserBookingsRequest) (*bookingpb.ListUserBookingsResponse, error) {
	if md, ok := metadata.FromIncomingContext(ctx); ok && len(md.Get("x-list-scope")) > 0 && md.Get("x-list-scope")[0] == "resource" {
		bookings, err := h.ctl.ListResourceBookings(ctx, req.GetUserId(), false)
		if err != nil {
			if err == entity.ErrInvalidResourceID {
				return nil, status.Error(codes.InvalidArgument, err.Error())
			}
			return nil, status.Error(codes.Internal, err.Error())
		}
		resp := &bookingpb.ListUserBookingsResponse{Bookings: make([]*bookingpb.GetBookingStatusResponse, 0, len(bookings))}
		for _, b := range bookings {
			resp.Bookings = append(resp.Bookings, util.BookingToStatusResponse(b))
		}
		return resp, nil
	}
	return h.listUserBookings(ctx, req.GetUserId(), false)
}

func (h *Handler) listUserBookings(ctx context.Context, userID string, upcoming bool) (*bookingpb.ListUserBookingsResponse, error) {
	bookings, err := h.ctl.ListUserBookings(ctx, userID, upcoming)
	if err != nil {
		if err == entity.ErrInvalidUserID {
			return nil, status.Error(codes.InvalidArgument, err.Error())
		}
		return nil, status.Error(codes.Internal, err.Error())
	}
	resp := &bookingpb.ListUserBookingsResponse{Bookings: make([]*bookingpb.GetBookingStatusResponse, 0, len(bookings))}
	for _, b := range bookings {
		resp.Bookings = append(resp.Bookings, util.BookingToStatusResponse(b))
	}
	return resp, nil
}

func (h *Handler) GetBookingStatus(ctx context.Context, req *bookingpb.GetBookingStatusRequest) (*bookingpb.GetBookingStatusResponse, error) {
	b, err := h.ctl.GetBooking(ctx, req.GetReferenceCode())
	if err != nil {
		if err == store.ErrNotFound {
			return nil, status.Error(codes.NotFound, "booking not found")
		}
		return nil, status.Error(codes.Internal, err.Error())
	}
	return util.BookingToStatusResponse(b), nil
}

func isValidationErr(err error) bool {
	switch err {
	case entity.ErrInvalidUserID, entity.ErrInvalidResourceID,
		entity.ErrInvalidTimeWindow, entity.ErrInvalidTitle, entity.ErrPastStartTime:
		return true
	default:
		return false
	}
}
