package controller

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/PriyanshuTrivedi/nexus-scheduler/code/booking/entity"
	clientmocks "github.com/PriyanshuTrivedi/nexus-scheduler/gen/mocks/booking/client"
	storemocks "github.com/PriyanshuTrivedi/nexus-scheduler/gen/mocks/booking/store"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
)

func setup(t *testing.T) (*storemocks.MockStore, *clientmocks.MockClient, Controller) {
	ctrl := gomock.NewController(t)

	mockStore := storemocks.NewMockStore(ctrl)
	mockClient := clientmocks.NewMockClient(ctrl)

	controller := New(mockStore, mockClient, nil)

	return mockStore, mockClient, controller
}

func validBooking() entity.Booking {
	start := time.Now().Add(time.Hour)
	return entity.Booking{
		UserID:        "user-1",
		ResourceID:    "resource-1",
		Start:         start,
		End:           start.Add(time.Hour),
		Title:         "Test booking",
		ReferenceCode: "NXS-123",
		Status:        entity.StatusConfirmed,
	}
}

func TestCreateBooking_Success(t *testing.T) {
	mockStore, mockClient, ctl := setup(t)

	booking := validBooking()

	mockClient.EXPECT().
		AcquireLock(gomock.Any(), booking.ResourceID).
		Return(func() {}, true, nil)

	mockStore.EXPECT().
		CreateBooking(gomock.Any(), gomock.Any()).
		Return(&booking, nil)

	mockClient.EXPECT().
		PublishEvent(gomock.Any(), "booking.created", booking.ReferenceCode).
		Return(nil)

	result, err := ctl.CreateBooking(context.Background(), booking)

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, booking.ReferenceCode, result.ReferenceCode)
}

func TestCreateBooking_LockError(t *testing.T) {
	_, mockClient, ctl := setup(t)

	booking := validBooking()
	lockErr := errors.New("redis unavailable")

	mockClient.EXPECT().
		AcquireLock(gomock.Any(), booking.ResourceID).
		Return(nil, false, lockErr)

	result, err := ctl.CreateBooking(context.Background(), booking)

	assert.ErrorIs(t, err, lockErr)
	assert.Nil(t, result)
}

func TestCreateBooking_SlotAlreadyBooked(t *testing.T) {
	_, mockClient, ctl := setup(t)

	booking := validBooking()

	mockClient.EXPECT().
		AcquireLock(gomock.Any(), booking.ResourceID).
		Return(nil, false, nil)

	result, err := ctl.CreateBooking(context.Background(), booking)

	assert.Error(t, err)
	assert.Nil(t, result)
}

func TestCreateBooking_StoreError(t *testing.T) {
	mockStore, mockClient, ctl := setup(t)

	booking := validBooking()
	storeErr := errors.New("database error")

	mockClient.EXPECT().
		AcquireLock(gomock.Any(), booking.ResourceID).
		Return(func() {}, true, nil)

	mockStore.EXPECT().
		CreateBooking(gomock.Any(), gomock.Any()).
		Return(nil, storeErr)

	result, err := ctl.CreateBooking(context.Background(), booking)

	assert.ErrorIs(t, err, storeErr)
	assert.Nil(t, result)
}

func TestCreateBooking_InvalidBooking(t *testing.T) {
	_, _, ctl := setup(t)

	booking := validBooking()
	booking.UserID = ""

	result, err := ctl.CreateBooking(context.Background(), booking)

	assert.Error(t, err)
	assert.Nil(t, result)
}

func TestCancelBooking_Success(t *testing.T) {
	mockStore, mockClient, ctl := setup(t)

	mockStore.EXPECT().
		CancelBooking(gomock.Any(), "NXS-CANCEL").
		Return(entity.StatusCancelled, nil)

	mockClient.EXPECT().
		PublishEvent(gomock.Any(), "booking.cancelled", "NXS-CANCEL").
		Return(nil)

	status, err := ctl.CancelBooking(context.Background(), "NXS-CANCEL")

	assert.NoError(t, err)
	assert.Equal(t, entity.StatusCancelled, status)
}

func TestCancelBooking_StoreError(t *testing.T) {
	mockStore, _, ctl := setup(t)

	storeErr := errors.New("database error")

	mockStore.EXPECT().
		CancelBooking(gomock.Any(), "NXS-CANCEL").
		Return(entity.StatusUnspecified, storeErr)

	status, err := ctl.CancelBooking(context.Background(), "NXS-CANCEL")

	assert.ErrorIs(t, err, storeErr)
	assert.Equal(t, entity.StatusUnspecified, status)
}

func TestRescheduleBooking_Success(t *testing.T) {
	mockStore, mockClient, ctl := setup(t)

	newStart := time.Now().Add(2 * time.Hour).Unix()
	newEnd := time.Now().Add(3 * time.Hour).Unix()

	booking := &entity.Booking{
		ReferenceCode: "NXS-RESCHEDULE",
		UserID:        "user-1",
		ResourceID:    "resource-1",
		Start:         time.Unix(newStart, 0),
		End:           time.Unix(newEnd, 0),
		Status:        entity.StatusRescheduled,
	}

	mockStore.EXPECT().
		RescheduleBooking(
			gomock.Any(),
			"NXS-RESCHEDULE",
			int64(newStart),
			int64(newEnd),
		).
		Return(booking, nil)

	mockClient.EXPECT().
		PublishEvent(gomock.Any(), "booking.rescheduled", "NXS-RESCHEDULE").
		Return(nil)

	result, err := ctl.RescheduleBooking(
		context.Background(),
		"NXS-RESCHEDULE",
		int64(newStart),
		int64(newEnd),
	)

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, time.Unix(newStart, 0), result.Start)
	assert.Equal(t, time.Unix(newEnd, 0), result.End)
}

func TestRescheduleBooking_StoreError(t *testing.T) {
	mockStore, _, ctl := setup(t)

	storeErr := errors.New("database error")

	newStart := time.Now().Add(2 * time.Hour).Unix()
	newEnd := time.Now().Add(3 * time.Hour).Unix()

	mockStore.EXPECT().
		RescheduleBooking(
			gomock.Any(),
			"NXS-RESCHEDULE",
			int64(newStart),
			int64(newEnd),
		).
		Return(nil, storeErr)

	result, err := ctl.RescheduleBooking(
		context.Background(),
		"NXS-RESCHEDULE",
		int64(newStart),
		int64(newEnd),
	)

	assert.ErrorIs(t, err, storeErr)
	assert.Nil(t, result)
}

func TestRescheduleBooking_PreservesReferenceCode(t *testing.T) {
	mockStore, mockClient, ctl := setup(t)

	newStart := time.Now().Add(2 * time.Hour).Unix()
	newEnd := time.Now().Add(3 * time.Hour).Unix()

	booking := &entity.Booking{
		ReferenceCode: "NXS-REF",
		UserID:        "user-1",
		ResourceID:    "resource-1",
		Start:         time.Unix(newStart, 0),
		End:           time.Unix(newEnd, 0),
		Status:        entity.StatusRescheduled,
	}

	mockStore.EXPECT().
		RescheduleBooking(
			gomock.Any(),
			"NXS-REF",
			int64(newStart),
			int64(newEnd),
		).
		Return(booking, nil)

	mockClient.EXPECT().
		PublishEvent(gomock.Any(), "booking.rescheduled", "NXS-REF").
		Return(nil)

	result, err := ctl.RescheduleBooking(
		context.Background(),
		"NXS-REF",
		int64(newStart),
		int64(newEnd),
	)

	assert.NoError(t, err)
	assert.Equal(t, "NXS-REF", result.ReferenceCode)
	assert.Equal(t, entity.StatusRescheduled, result.Status)
}
