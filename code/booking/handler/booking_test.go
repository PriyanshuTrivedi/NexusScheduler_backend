package handler

import (
	"context"
	"testing"

	"github.com/PriyanshuTrivedi/nexus-scheduler/code/booking/entity"
	"github.com/PriyanshuTrivedi/nexus-scheduler/code/booking/store"
	"github.com/PriyanshuTrivedi/nexus-scheduler/code/booking/util"
	bookingpb "github.com/PriyanshuTrivedi/nexus-scheduler/gen/idl/booking"
	ctrlmocks "github.com/PriyanshuTrivedi/nexus-scheduler/gen/mocks/booking/controller"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

func handlerContext() context.Context {
	return metadata.NewIncomingContext(context.Background(), metadata.Pairs(
		"x-booking-client-name", "Priyanshu", "x-booking-client-email", "client@example.com",
		"x-booking-resource-name", "Suyash Gupta", "x-booking-resource-email", "resource@example.com",
		"x-booking-meeting-mode", "online", "x-booking-address", "Bangalore",
		"x-booking-meeting-link", "https://meet.jit.si/NXS-ABC123",
	))
}

func TestCreateBooking_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockCtl := ctrlmocks.NewMockController(ctrl)
	h := New(mockCtl)
	mockCtl.EXPECT().CreateBooking(gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, b entity.Booking) (*entity.Booking, error) {
		n, ok := util.BookingNotificationContextFromContext(ctx)
		assert.True(t, ok)
		assert.Equal(t, "client@example.com", n.ClientEmail)
		assert.Equal(t, "resource@example.com", n.ResourceEmail)
		assert.Equal(t, "online", n.MeetingMode)
		return &entity.Booking{ReferenceCode: "NXS-ABC123", Status: entity.StatusConfirmed}, nil
	})
	resp, err := h.CreateBooking(handlerContext(), &bookingpb.CreateBookingRequest{UserId: "u1", ResourceId: "r1", Title: "x", StartUnix: 1900000000, EndUnix: 1900003600})
	assert.NoError(t, err)
	assert.Equal(t, "NXS-ABC123", resp.ReferenceCode)
}

func TestCreateBooking_ValidationError(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockCtl := ctrlmocks.NewMockController(ctrl)
	h := New(mockCtl)
	mockCtl.EXPECT().CreateBooking(gomock.Any(), gomock.Any()).Return(nil, entity.ErrInvalidTitle)
	_, err := h.CreateBooking(context.Background(), &bookingpb.CreateBookingRequest{})
	assert.Equal(t, codes.InvalidArgument, status.Code(err))
}

func TestCreateBooking_AlreadyBooked_MapsToAlreadyExists(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockCtl := ctrlmocks.NewMockController(ctrl)
	h := New(mockCtl)
	mockCtl.EXPECT().CreateBooking(gomock.Any(), gomock.Any()).Return(nil, store.ErrSlotAlreadyBooked)
	_, err := h.CreateBooking(context.Background(), &bookingpb.CreateBookingRequest{UserId: "u1", ResourceId: "r1", Title: "x", StartUnix: 1900000000, EndUnix: 1900003600})
	assert.Equal(t, codes.AlreadyExists, status.Code(err))
}

func TestCreateBooking_InternalError(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockCtl := ctrlmocks.NewMockController(ctrl)
	h := New(mockCtl)
	mockCtl.EXPECT().CreateBooking(gomock.Any(), gomock.Any()).Return(nil, assert.AnError)
	_, err := h.CreateBooking(context.Background(), &bookingpb.CreateBookingRequest{UserId: "u1", ResourceId: "r1", Title: "x", StartUnix: 1900000000, EndUnix: 1900003600})
	assert.Equal(t, codes.Internal, status.Code(err))
}

func TestCancelBooking_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockCtl := ctrlmocks.NewMockController(ctrl)
	h := New(mockCtl)
	mockCtl.EXPECT().CancelBooking(gomock.Any(), "NXS-ABC123").DoAndReturn(func(ctx context.Context, ref string) (entity.BookingStatus, error) {
		n, ok := util.BookingNotificationContextFromContext(ctx)
		assert.True(t, ok)
		assert.Equal(t, "online", n.MeetingMode)
		return entity.StatusCancelled, nil
	})
	resp, err := h.CancelBooking(handlerContext(), &bookingpb.CancelBookingRequest{ReferenceCode: "NXS-ABC123"})
	assert.NoError(t, err)
	assert.Equal(t, bookingpb.BookingStatus_BOOKING_STATUS_CANCELLED, resp.Status)
}

func TestCancelBooking_NotFound(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockCtl := ctrlmocks.NewMockController(ctrl)
	h := New(mockCtl)
	mockCtl.EXPECT().CancelBooking(gomock.Any(), "NXS-NOPE").Return(entity.StatusUnspecified, store.ErrNotFound)
	_, err := h.CancelBooking(context.Background(), &bookingpb.CancelBookingRequest{ReferenceCode: "NXS-NOPE"})
	assert.Equal(t, codes.NotFound, status.Code(err))
}

func TestCancelBooking_InternalError(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockCtl := ctrlmocks.NewMockController(ctrl)
	h := New(mockCtl)
	mockCtl.EXPECT().CancelBooking(gomock.Any(), "NXS-FAIL").Return(entity.StatusUnspecified, assert.AnError)
	_, err := h.CancelBooking(context.Background(), &bookingpb.CancelBookingRequest{ReferenceCode: "NXS-FAIL"})
	assert.Equal(t, codes.Internal, status.Code(err))
}

func TestRescheduleBooking_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockCtl := ctrlmocks.NewMockController(ctrl)
	h := New(mockCtl)
	mockCtl.EXPECT().RescheduleBooking(gomock.Any(), "NXS-ABC123", int64(1900000000), int64(1900003600)).DoAndReturn(func(ctx context.Context, ref string, start, end int64) (*entity.Booking, error) {
		n, ok := util.BookingNotificationContextFromContext(ctx)
		assert.True(t, ok)
		assert.Equal(t, "resource@example.com", n.ResourceEmail)
		return &entity.Booking{ReferenceCode: ref, Status: entity.StatusRescheduled}, nil
	})
	resp, err := h.RescheduleBooking(handlerContext(), &bookingpb.RescheduleBookingRequest{ReferenceCode: "NXS-ABC123", StartUnix: 1900000000, EndUnix: 1900003600})
	assert.NoError(t, err)
	assert.Equal(t, bookingpb.BookingStatus_BOOKING_STATUS_RESCHEDULED, resp.Status)
}

func TestRescheduleBooking_Errors(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want codes.Code
	}{
		{"not found", store.ErrNotFound, codes.NotFound},
		{"slot taken", store.ErrSlotAlreadyBooked, codes.AlreadyExists},
		{"internal", assert.AnError, codes.Internal},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			mockCtl := ctrlmocks.NewMockController(ctrl)
			h := New(mockCtl)
			mockCtl.EXPECT().RescheduleBooking(gomock.Any(), "NXS-1", int64(1900000000), int64(1900003600)).Return(nil, tt.err)
			_, err := h.RescheduleBooking(context.Background(), &bookingpb.RescheduleBookingRequest{ReferenceCode: "NXS-1", StartUnix: 1900000000, EndUnix: 1900003600})
			assert.Equal(t, tt.want, status.Code(err))
		})
	}
}

func TestListUpcomingBookings_UserScope(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockCtl := ctrlmocks.NewMockController(ctrl)
	h := New(mockCtl)
	mockCtl.EXPECT().ListUserBookings(gomock.Any(), "u1", true).Return([]*entity.Booking{{ReferenceCode: "NXS-1", Status: entity.StatusConfirmed}}, nil)
	resp, err := h.ListUpcomingBookings(context.Background(), &bookingpb.ListUserBookingsRequest{UserId: "u1"})
	assert.NoError(t, err)
	assert.Len(t, resp.GetBookings(), 1)
}

func TestListUpcomingBookings_ResourceScope(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockCtl := ctrlmocks.NewMockController(ctrl)
	h := New(mockCtl)
	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs("x-list-scope", "resource"))
	mockCtl.EXPECT().ListResourceBookings(gomock.Any(), "r1", true).Return([]*entity.Booking{{ReferenceCode: "NXS-1"}}, nil)
	resp, err := h.ListUpcomingBookings(ctx, &bookingpb.ListUserBookingsRequest{UserId: "r1"})
	assert.NoError(t, err)
	assert.Len(t, resp.GetBookings(), 1)
}

func TestListUpcomingBookings_ResourceInvalidID(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockCtl := ctrlmocks.NewMockController(ctrl)
	h := New(mockCtl)
	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs("x-list-scope", "resource"))
	mockCtl.EXPECT().ListResourceBookings(gomock.Any(), "", true).Return(nil, entity.ErrInvalidResourceID)
	_, err := h.ListUpcomingBookings(ctx, &bookingpb.ListUserBookingsRequest{})
	assert.Equal(t, codes.InvalidArgument, status.Code(err))
}

func TestListPastBookings_UserAndResourceScopes(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockCtl := ctrlmocks.NewMockController(ctrl)
	h := New(mockCtl)
	mockCtl.EXPECT().ListUserBookings(gomock.Any(), "u1", false).Return(nil, nil)
	_, err := h.ListPastBookings(context.Background(), &bookingpb.ListUserBookingsRequest{UserId: "u1"})
	assert.NoError(t, err)
	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs("x-list-scope", "resource"))
	mockCtl.EXPECT().ListResourceBookings(gomock.Any(), "r1", false).Return(nil, nil)
	_, err = h.ListPastBookings(ctx, &bookingpb.ListUserBookingsRequest{UserId: "r1"})
	assert.NoError(t, err)
}

func TestListBookings_InternalErrors(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockCtl := ctrlmocks.NewMockController(ctrl)
	h := New(mockCtl)
	mockCtl.EXPECT().ListUserBookings(gomock.Any(), "u1", true).Return(nil, assert.AnError)
	_, err := h.ListUpcomingBookings(context.Background(), &bookingpb.ListUserBookingsRequest{UserId: "u1"})
	assert.Equal(t, codes.Internal, status.Code(err))
	mockCtl.EXPECT().ListResourceBookings(gomock.Any(), "r1", false).Return(nil, assert.AnError)
	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs("x-list-scope", "resource"))
	_, err = h.ListPastBookings(ctx, &bookingpb.ListUserBookingsRequest{UserId: "r1"})
	assert.Equal(t, codes.Internal, status.Code(err))
}

func TestGetBookingStatus(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockCtl := ctrlmocks.NewMockController(ctrl)
	h := New(mockCtl)
	b := &entity.Booking{ReferenceCode: "NXS-1", UserID: "u1", ResourceID: "r1", Title: "Interview", Status: entity.StatusConfirmed}
	mockCtl.EXPECT().GetBooking(gomock.Any(), "NXS-1").Return(b, nil)
	resp, err := h.GetBookingStatus(context.Background(), &bookingpb.GetBookingStatusRequest{ReferenceCode: "NXS-1"})
	assert.NoError(t, err)
	assert.Equal(t, "NXS-1", resp.ReferenceCode)
	mockCtl.EXPECT().GetBooking(gomock.Any(), "NXS-NOPE").Return(nil, store.ErrNotFound)
	_, err = h.GetBookingStatus(context.Background(), &bookingpb.GetBookingStatusRequest{ReferenceCode: "NXS-NOPE"})
	assert.Equal(t, codes.NotFound, status.Code(err))
}

func TestIsResourceScope(t *testing.T) {
	assert.False(t, isResourceScope(context.Background()))
	assert.False(t, isResourceScope(metadata.NewIncomingContext(context.Background(), metadata.Pairs("x-list-scope", "client"))))
	assert.True(t, isResourceScope(metadata.NewIncomingContext(context.Background(), metadata.Pairs("x-list-scope", "resource"))))
}
