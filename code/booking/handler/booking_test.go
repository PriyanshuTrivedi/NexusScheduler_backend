package handler

import (
	"context"
	"testing"

	bookingpb "github.com/PriyanshuTrivedi/nexus-scheduler/gen/idl/booking"

	"github.com/PriyanshuTrivedi/nexus-scheduler/code/booking/entity"
	"github.com/PriyanshuTrivedi/nexus-scheduler/code/booking/store"
	ctrlmocks "github.com/PriyanshuTrivedi/nexus-scheduler/gen/mocks/booking/controller"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestCreateBooking_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockCtl := ctrlmocks.NewMockController(ctrl)
	h := New(mockCtl)

	mockCtl.EXPECT().
		CreateBooking(gomock.Any(), gomock.Any()).
		Return(&entity.Booking{ReferenceCode: "NXS-ABC123", Status: entity.StatusConfirmed}, nil)

	resp, err := h.CreateBooking(context.Background(), &bookingpb.CreateBookingRequest{
		UserId: "u1", ResourceId: "r1", Title: "x", StartUnix: 1900000000, EndUnix: 1900003600,
	})

	assert.NoError(t, err)
	assert.Equal(t, "NXS-ABC123", resp.ReferenceCode)
}

func TestCreateBooking_AlreadyBooked_MapsToAlreadyExists(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockCtl := ctrlmocks.NewMockController(ctrl)
	h := New(mockCtl)

	mockCtl.EXPECT().CreateBooking(gomock.Any(), gomock.Any()).Return(nil, store.ErrSlotAlreadyBooked)

	_, err := h.CreateBooking(context.Background(), &bookingpb.CreateBookingRequest{
		UserId: "u1", ResourceId: "r1", Title: "x", StartUnix: 1900000000, EndUnix: 1900003600,
	})

	assert.Equal(t, codes.AlreadyExists, status.Code(err))
}

func TestCancelBooking_NotFound_MapsToNotFound(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockCtl := ctrlmocks.NewMockController(ctrl)
	h := New(mockCtl)

	mockCtl.EXPECT().CancelBooking(gomock.Any(), "NXS-NOPE").Return(entity.StatusUnspecified, store.ErrNotFound)

	_, err := h.CancelBooking(context.Background(), &bookingpb.CancelBookingRequest{ReferenceCode: "NXS-NOPE"})

	assert.Equal(t, codes.NotFound, status.Code(err))
}

func TestGetBookingStatus_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockCtl := ctrlmocks.NewMockController(ctrl)
	h := New(mockCtl)

	mockCtl.EXPECT().
		GetBooking(gomock.Any(), "NXS-ABC123").
		Return(&entity.Booking{ReferenceCode: "NXS-ABC123", Status: entity.StatusConfirmed}, nil)

	resp, err := h.GetBookingStatus(context.Background(), &bookingpb.GetBookingStatusRequest{ReferenceCode: "NXS-ABC123"})

	assert.NoError(t, err)
	assert.Equal(t, "NXS-ABC123", resp.ReferenceCode)
}
