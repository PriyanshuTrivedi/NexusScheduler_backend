package controller

import (
	"context"
	"testing"
	"time"

	"github.com/PriyanshuTrivedi/nexus-scheduler/code/booking/entity"
	"github.com/PriyanshuTrivedi/nexus-scheduler/code/booking/mailer"
	"github.com/PriyanshuTrivedi/nexus-scheduler/code/booking/store"
	clientmocks "github.com/PriyanshuTrivedi/nexus-scheduler/gen/mocks/booking/client"
	storemocks "github.com/PriyanshuTrivedi/nexus-scheduler/gen/mocks/booking/store"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
)

func setup(t *testing.T) (*storemocks.MockStore, *clientmocks.MockClient, Controller) {
	ctrl := gomock.NewController(t)
	mockStore := storemocks.NewMockStore(ctrl)
	mockClient := clientmocks.NewMockClient(ctrl)
	return mockStore, mockClient, New(mockStore, mockClient, &mailer.ConsoleMailer{})
}

func TestCreateBooking_Success(t *testing.T) {
	mockStore, mockClient, c := setup(t)

	input := entity.Booking{
		UserID: "u1", ResourceID: "r1", Title: "ENT consult",
		Start: time.Now().Add(time.Hour), End: time.Now().Add(2 * time.Hour),
	}
	expected := input
	expected.ReferenceCode, expected.Status = "NXS-ABC123", entity.StatusConfirmed

	release := func() {}
	mockClient.EXPECT().AcquireLock(gomock.Any(), "r1").Return(release, true, nil)
	mockStore.EXPECT().CreateBooking(gomock.Any(), input).Return(&expected, nil)
	mockClient.EXPECT().PublishEvent(gomock.Any(), "booking.created", "NXS-ABC123").Return(nil)

	result, err := c.CreateBooking(context.Background(), input)

	assert.NoError(t, err)
	assert.Equal(t, "NXS-ABC123", result.ReferenceCode)
}

func TestCreateBooking_RejectsInvalidInput(t *testing.T) {
	_, _, c := setup(t) // no mock expectations set — validation must fail before either is touched

	_, err := c.CreateBooking(context.Background(), entity.Booking{
		UserID: "u1", ResourceID: "r1", Title: "x",
		Start: time.Now().Add(-time.Hour), End: time.Now(),
	})
	assert.ErrorIs(t, err, entity.ErrPastStartTime)
}

func TestCreateBooking_LockContended(t *testing.T) {
	mockStore, mockClient, c := setup(t)
	_ = mockStore // unused in this path — CreateBooking must never be called if the lock fails

	mockClient.EXPECT().AcquireLock(gomock.Any(), "r1").Return(nil, false, nil)

	input := entity.Booking{
		UserID: "u1", ResourceID: "r1", Title: "x",
		Start: time.Now().Add(time.Hour), End: time.Now().Add(2 * time.Hour),
	}
	_, err := c.CreateBooking(context.Background(), input)
	assert.ErrorIs(t, err, store.ErrSlotAlreadyBooked)
}

func TestCancelBooking_Success(t *testing.T) {
	mockStore, mockClient, c := setup(t)

	mockStore.EXPECT().CancelBooking(gomock.Any(), "NXS-ABC123").Return(entity.StatusCancelled, nil)
	mockClient.EXPECT().PublishEvent(gomock.Any(), "booking.cancelled", "NXS-ABC123").Return(nil)

	status, err := c.CancelBooking(context.Background(), "NXS-ABC123")

	assert.NoError(t, err)
	assert.Equal(t, entity.StatusCancelled, status)
}

func TestCancelBooking_NotFound(t *testing.T) {
	mockStore, _, c := setup(t)

	mockStore.EXPECT().CancelBooking(gomock.Any(), "NXS-NOPE").Return(entity.StatusUnspecified, store.ErrNotFound)

	_, err := c.CancelBooking(context.Background(), "NXS-NOPE")
	assert.ErrorIs(t, err, store.ErrNotFound)
}

func TestGetBooking_Success(t *testing.T) {
	mockStore, _, c := setup(t)

	expected := &entity.Booking{ReferenceCode: "NXS-ABC123", Status: entity.StatusConfirmed}
	mockStore.EXPECT().GetBooking(gomock.Any(), "NXS-ABC123").Return(expected, nil)

	result, err := c.GetBooking(context.Background(), "NXS-ABC123")

	assert.NoError(t, err)
	assert.Equal(t, expected, result)
}
