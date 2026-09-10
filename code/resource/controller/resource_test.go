package controller

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"

	"github.com/PriyanshuTrivedi/nexus-scheduler/code/resource/entity"
	clientmocks "github.com/PriyanshuTrivedi/nexus-scheduler/gen/mocks/resource/client"
	locationIQmocks "github.com/PriyanshuTrivedi/nexus-scheduler/gen/mocks/resource/client/locationIQ"
	storemocks "github.com/PriyanshuTrivedi/nexus-scheduler/gen/mocks/resource/store"
)

func validResource() entity.Resource {
	return entity.Resource{
		TenantType:     entity.TenantTypeOrg,
		OrgID:          "org-1",
		ResourceTypeID: "type-1",
		Name:           "Dr. Rakesh",
		MeetingMode:    entity.MeetingModeOnline,
	}
}

func TestCreateResourceType_ValidationAndStore(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockStore := storemocks.NewMockStore(ctrl)

	c := New(
		mockStore,
		clientmocks.NewMockClient(ctrl),
		locationIQmocks.NewMockLocationIQClient(ctrl),
	)

	_, err := c.CreateResourceType(context.Background(), "")
	assert.ErrorIs(t, err, entity.ErrInvalidResourceTypeName)

	mockStore.EXPECT().
		CreateResourceType(gomock.Any(), "doctor").
		Return(entity.ResourceType{
			ID:   "type-1",
			Name: "doctor",
		}, nil)

	got, err := c.CreateResourceType(context.Background(), "doctor")

	assert.NoError(t, err)
	assert.Equal(t, "type-1", got.ID)
}

func TestDeleteResourceType_Validation(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockStore := storemocks.NewMockStore(ctrl)

	c := New(
		mockStore,
		clientmocks.NewMockClient(ctrl),
		locationIQmocks.NewMockLocationIQClient(ctrl),
	)

	assert.ErrorIs(
		t,
		c.DeleteResourceType(context.Background(), ""),
		entity.ErrInvalidResourceTypeID,
	)

	mockStore.EXPECT().
		DeleteResourceType(gomock.Any(), "type-1").
		Return(nil)

	assert.NoError(
		t,
		c.DeleteResourceType(context.Background(), "type-1"),
	)
}

func TestCreateResource_ExpandsRecurrenceAndInvalidatesCache(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockStore := storemocks.NewMockStore(ctrl)
	mockClient := clientmocks.NewMockClient(ctrl)
	mockLocationIQ := locationIQmocks.NewMockLocationIQClient(ctrl)

	c := New(mockStore, mockClient, mockLocationIQ)

	r := validResource()
	r.Recurrence = []entity.RecurrenceRule{
		{
			Day:      entity.Monday,
			Timezone: "Asia/Kolkata",
			Slots: []entity.TimeSlot{
				{
					StartHour: 9,
					EndHour:   17,
				},
			},
		},
	}

	mockStore.EXPECT().
		CreateResource(
			gomock.Any(),
			gomock.Any(),
			gomock.Not(gomock.Nil()),
		).
		DoAndReturn(
			func(
				_ context.Context,
				_ entity.Resource,
				slots []entity.Slot,
			) (string, error) {
				assert.NotEmpty(t, slots)
				return "res-1", nil
			},
		)

	mockClient.EXPECT().
		InvalidateOrgSearchCache(gomock.Any(), "org-1").
		Return(nil)

	id, err := c.CreateResource(context.Background(), r)

	assert.NoError(t, err)
	assert.Equal(t, "res-1", id)
}

func TestCreateResource_OfflineGeocodesAddress(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockStore := storemocks.NewMockStore(ctrl)
	mockClient := clientmocks.NewMockClient(ctrl)
	mockLocationIQ := locationIQmocks.NewMockLocationIQClient(ctrl)

	c := New(mockStore, mockClient, mockLocationIQ)

	address := "Bangalore, India"

	r := validResource()
	r.MeetingMode = entity.MeetingModeOffline
	r.Address = &address
	r.Attributes = map[string]string{
		"address": address,
	}

	coordinates := []entity.Coordinate{
		{
			Latitude:  12.9716,
			Longitude: 77.5946,
		},
	}

	mockLocationIQ.EXPECT().
		GetCoordinates(gomock.Any(), address).
		Return(coordinates, nil)

	mockStore.EXPECT().
		CreateResource(
			gomock.Any(),
			gomock.Any(),
			gomock.Nil(),
		).
		DoAndReturn(
			func(
				_ context.Context,
				resource entity.Resource,
				_ []entity.Slot,
			) (string, error) {
				assert.NotNil(t, resource.Coordinate)
				assert.Equal(
					t,
					12.9716,
					resource.Coordinate.Latitude,
				)
				assert.Equal(
					t,
					77.5946,
					resource.Coordinate.Longitude,
				)
				assert.Equal(
					t,
					address,
					resource.Attributes["address"],
				)

				return "res-1", nil
			},
		)

	mockClient.EXPECT().
		InvalidateOrgSearchCache(gomock.Any(), "org-1").
		Return(nil)

	id, err := c.CreateResource(context.Background(), r)

	assert.NoError(t, err)
	assert.Equal(t, "res-1", id)
}

func TestCreateResource_OfflineWithoutAddressReturnsLocationRequired(t *testing.T) {
	ctrl := gomock.NewController(t)

	c := New(
		storemocks.NewMockStore(ctrl),
		clientmocks.NewMockClient(ctrl),
		locationIQmocks.NewMockLocationIQClient(ctrl),
	)

	r := validResource()
	r.MeetingMode = entity.MeetingModeOffline

	_, err := c.CreateResource(context.Background(), r)

	assert.ErrorIs(t, err, entity.ErrLocationRequired)
}

func TestCreateResource_GeocodingError(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockStore := storemocks.NewMockStore(ctrl)
	mockClient := clientmocks.NewMockClient(ctrl)
	mockLocationIQ := locationIQmocks.NewMockLocationIQClient(ctrl)

	c := New(mockStore, mockClient, mockLocationIQ)

	address := "Bangalore, India"

	r := validResource()
	r.MeetingMode = entity.MeetingModeOffline
	r.Address = &address
	r.Attributes = map[string]string{
		"address": address,
	}

	expectedErr := errors.New("location service unavailable")

	mockLocationIQ.EXPECT().
		GetCoordinates(gomock.Any(), address).
		Return(nil, expectedErr)

	_, err := c.CreateResource(context.Background(), r)

	assert.ErrorIs(t, err, expectedErr)
}

func TestCreateResource_GeocodingReturnsNoCoordinates(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockStore := storemocks.NewMockStore(ctrl)
	mockClient := clientmocks.NewMockClient(ctrl)
	mockLocationIQ := locationIQmocks.NewMockLocationIQClient(ctrl)

	c := New(mockStore, mockClient, mockLocationIQ)

	address := "Unknown Location"

	r := validResource()
	r.MeetingMode = entity.MeetingModeOffline
	r.Address = &address

	mockLocationIQ.EXPECT().
		GetCoordinates(gomock.Any(), address).
		Return([]entity.Coordinate{}, nil)

	_, err := c.CreateResource(context.Background(), r)

	assert.ErrorIs(t, err, entity.ErrLocationRequired)
}

func TestUpdateResource_DelegatesToStoreAndInvalidatesCache(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockStore := storemocks.NewMockStore(ctrl)
	mockClient := clientmocks.NewMockClient(ctrl)
	mockLocationIQ := locationIQmocks.NewMockLocationIQClient(ctrl)

	c := New(mockStore, mockClient, mockLocationIQ)

	r := entity.Resource{
		ID:          "res-1",
		Name:        "Dr. Rakesh Updated",
		TenantType:  entity.TenantTypeOrg,
		OrgID:       "org-1",
		MeetingMode: entity.MeetingModeOnline,
		Attributes:  map[string]string{},
	}

	mockStore.EXPECT().
		UpdateResource(gomock.Any(), r).
		Return("res-1", nil)

	orgID := "org-1"

	mockStore.EXPECT().
		GetResourceOrgID(gomock.Any(), "res-1").
		Return(&orgID, nil)

	mockClient.EXPECT().
		InvalidateOrgSearchCache(gomock.Any(), "org-1").
		Return(nil)

	id, err := c.UpdateResource(context.Background(), r)

	assert.NoError(t, err)
	assert.Equal(t, "res-1", id)
}

func TestUpdateResource_GeocodesAddress(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockStore := storemocks.NewMockStore(ctrl)
	mockClient := clientmocks.NewMockClient(ctrl)
	mockLocationIQ := locationIQmocks.NewMockLocationIQClient(ctrl)

	c := New(mockStore, mockClient, mockLocationIQ)

	address := "Bangalore, India"

	r := entity.Resource{
		ID:          "res-1",
		Name:        "Dr. Rakesh",
		TenantType:  entity.TenantTypeOrg,
		OrgID:       "org-1",
		MeetingMode: entity.MeetingModeOffline,
		Address:     &address,
		Attributes: map[string]string{
			"address": address,
		},
	}

	coordinates := []entity.Coordinate{
		{
			Latitude:  12.9716,
			Longitude: 77.5946,
		},
	}

	mockLocationIQ.EXPECT().
		GetCoordinates(gomock.Any(), address).
		Return(coordinates, nil)

	mockStore.EXPECT().
		UpdateResource(
			gomock.Any(),
			gomock.Any(),
		).
		DoAndReturn(
			func(
				_ context.Context,
				resource entity.Resource,
			) (string, error) {
				assert.NotNil(t, resource.Coordinate)
				assert.Equal(
					t,
					12.9716,
					resource.Coordinate.Latitude,
				)
				assert.Equal(
					t,
					77.5946,
					resource.Coordinate.Longitude,
				)
				assert.Equal(
					t,
					address,
					resource.Attributes["address"],
				)

				return "res-1", nil
			},
		)

	orgID := "org-1"

	mockStore.EXPECT().
		GetResourceOrgID(gomock.Any(), "res-1").
		Return(&orgID, nil)

	mockClient.EXPECT().
		InvalidateOrgSearchCache(gomock.Any(), "org-1").
		Return(nil)

	id, err := c.UpdateResource(context.Background(), r)

	assert.NoError(t, err)
	assert.Equal(t, "res-1", id)
}

func TestUpdateResource_WithoutAddressDoesNotGeocode(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockStore := storemocks.NewMockStore(ctrl)
	mockClient := clientmocks.NewMockClient(ctrl)
	mockLocationIQ := locationIQmocks.NewMockLocationIQClient(ctrl)

	c := New(mockStore, mockClient, mockLocationIQ)

	r := entity.Resource{
		ID:          "res-1",
		Name:        "Dr. Rakesh",
		TenantType:  entity.TenantTypeOrg,
		OrgID:       "org-1",
		MeetingMode: entity.MeetingModeOnline,
		Attributes:  map[string]string{},
	}

	mockStore.EXPECT().
		UpdateResource(gomock.Any(), r).
		Return("res-1", nil)

	orgID := "org-1"

	mockStore.EXPECT().
		GetResourceOrgID(gomock.Any(), "res-1").
		Return(&orgID, nil)

	mockClient.EXPECT().
		InvalidateOrgSearchCache(gomock.Any(), "org-1").
		Return(nil)

	id, err := c.UpdateResource(context.Background(), r)

	assert.NoError(t, err)
	assert.Equal(t, "res-1", id)
}

func TestUpdateResource_OfflineWithoutAddressReturnsLocationRequired(t *testing.T) {
	ctrl := gomock.NewController(t)

	c := New(
		storemocks.NewMockStore(ctrl),
		clientmocks.NewMockClient(ctrl),
		locationIQmocks.NewMockLocationIQClient(ctrl),
	)

	r := entity.Resource{
		ID:          "res-1",
		Name:        "Dr. Rakesh",
		TenantType:  entity.TenantTypeOrg,
		OrgID:       "org-1",
		MeetingMode: entity.MeetingModeOffline,
	}

	_, err := c.UpdateResource(context.Background(), r)

	assert.ErrorIs(t, err, entity.ErrLocationRequired)
}

func TestUpdateResource_GeocodingError(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockStore := storemocks.NewMockStore(ctrl)
	mockClient := clientmocks.NewMockClient(ctrl)
	mockLocationIQ := locationIQmocks.NewMockLocationIQClient(ctrl)

	c := New(mockStore, mockClient, mockLocationIQ)

	address := "Bangalore, India"

	r := entity.Resource{
		ID:          "res-1",
		Name:        "Dr. Rakesh",
		TenantType:  entity.TenantTypeOrg,
		OrgID:       "org-1",
		MeetingMode: entity.MeetingModeOffline,
		Address:     &address,
	}

	expectedErr := errors.New("location service unavailable")

	mockLocationIQ.EXPECT().
		GetCoordinates(gomock.Any(), address).
		Return(nil, expectedErr)

	_, err := c.UpdateResource(context.Background(), r)

	assert.ErrorIs(t, err, expectedErr)
}

func TestUpdateResource_InvalidResourceID(t *testing.T) {
	ctrl := gomock.NewController(t)

	c := New(
		storemocks.NewMockStore(ctrl),
		clientmocks.NewMockClient(ctrl),
		locationIQmocks.NewMockLocationIQClient(ctrl),
	)

	r := entity.Resource{
		Name:        "Dr. Rakesh",
		TenantType:  entity.TenantTypeIndividual,
		MeetingMode: entity.MeetingModeOnline,
	}

	_, err := c.UpdateResource(context.Background(), r)

	assert.ErrorIs(t, err, entity.ErrInvalidResourceID)
}

func TestUpdateResource_InvalidName(t *testing.T) {
	ctrl := gomock.NewController(t)

	c := New(
		storemocks.NewMockStore(ctrl),
		clientmocks.NewMockClient(ctrl),
		locationIQmocks.NewMockLocationIQClient(ctrl),
	)

	r := entity.Resource{
		ID:          "res-1",
		TenantType:  entity.TenantTypeIndividual,
		MeetingMode: entity.MeetingModeOnline,
	}

	_, err := c.UpdateResource(context.Background(), r)

	assert.ErrorIs(t, err, entity.ErrInvalidName)
}

func TestUpdateResource_InvalidMeetingMode(t *testing.T) {
	ctrl := gomock.NewController(t)

	c := New(
		storemocks.NewMockStore(ctrl),
		clientmocks.NewMockClient(ctrl),
		locationIQmocks.NewMockLocationIQClient(ctrl),
	)

	r := entity.Resource{
		ID:         "res-1",
		Name:       "Dr. Rakesh",
		TenantType: entity.TenantTypeIndividual,
	}

	_, err := c.UpdateResource(context.Background(), r)

	assert.ErrorIs(t, err, entity.ErrInvalidMeetingMode)
}

func TestSetResourceStatus_InvalidResourceID(t *testing.T) {
	ctrl := gomock.NewController(t)

	c := New(
		storemocks.NewMockStore(ctrl),
		clientmocks.NewMockClient(ctrl),
		locationIQmocks.NewMockLocationIQClient(ctrl),
	)

	assert.ErrorIs(
		t,
		c.SetResourceStatus(context.Background(), "", false),
		entity.ErrInvalidResourceID,
	)
}

func TestSetResourceStatus_InvalidatesOrgCache(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockStore := storemocks.NewMockStore(ctrl)
	mockClient := clientmocks.NewMockClient(ctrl)

	c := New(
		mockStore,
		mockClient,
		locationIQmocks.NewMockLocationIQClient(ctrl),
	)

	orgID := "org-1"

	mockStore.EXPECT().
		SetResourceStatus(gomock.Any(), "res-1", false).
		Return(&orgID, nil)

	mockClient.EXPECT().
		InvalidateOrgSearchCache(gomock.Any(), "org-1").
		Return(nil)

	assert.NoError(
		t,
		c.SetResourceStatus(context.Background(), "res-1", false),
	)
}

func TestDeleteResource_OnlyDelegatesAndInvalidates(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockStore := storemocks.NewMockStore(ctrl)
	mockClient := clientmocks.NewMockClient(ctrl)

	c := New(
		mockStore,
		mockClient,
		locationIQmocks.NewMockLocationIQClient(ctrl),
	)

	orgID := "org-1"

	mockStore.EXPECT().
		DeleteResource(gomock.Any(), "res-1").
		Return(&orgID, nil)

	mockClient.EXPECT().
		InvalidateOrgSearchCache(gomock.Any(), "org-1").
		Return(nil)

	assert.NoError(
		t,
		c.DeleteResource(context.Background(), "res-1"),
	)
}

func TestDeleteResource_Validation(t *testing.T) {
	ctrl := gomock.NewController(t)

	c := New(
		storemocks.NewMockStore(ctrl),
		clientmocks.NewMockClient(ctrl),
		locationIQmocks.NewMockLocationIQClient(ctrl),
	)

	assert.ErrorIs(
		t,
		c.DeleteResource(context.Background(), ""),
		entity.ErrInvalidResourceID,
	)
}

func TestExpandRecurrence_NeverProducesPastSlots(t *testing.T) {
	ctrl := gomock.NewController(t)

	c := New(
		storemocks.NewMockStore(ctrl),
		clientmocks.NewMockClient(ctrl),
		locationIQmocks.NewMockLocationIQClient(ctrl),
	).(*resourceController)

	loc, _ := time.LoadLocation("Asia/Kolkata")

	from := time.Date(
		2026,
		9,
		7,
		0,
		0,
		0,
		0,
		loc,
	)

	rules := []entity.RecurrenceRule{
		{
			Day:      entity.Monday,
			Timezone: "Asia/Kolkata",
			Slots: []entity.TimeSlot{
				{
					StartHour: 0,
					EndHour:   1,
				},
			},
		},
	}

	slots, err := c.expandRecurrence(rules, from)

	assert.NoError(t, err)

	for _, s := range slots {
		assert.False(t, s.End.Before(time.Now()))
	}
}

func TestExpandRecurrence_BadTimezoneErrors(t *testing.T) {
	ctrl := gomock.NewController(t)

	c := New(
		storemocks.NewMockStore(ctrl),
		clientmocks.NewMockClient(ctrl),
		locationIQmocks.NewMockLocationIQClient(ctrl),
	).(*resourceController)

	rules := []entity.RecurrenceRule{
		{
			Day:      entity.Monday,
			Timezone: "Not/AZone",
			Slots: []entity.TimeSlot{
				{
					StartHour: 9,
					EndHour:   17,
				},
			},
		},
	}

	_, err := c.expandRecurrence(rules, time.Now())

	assert.Error(t, err)
}

func TestAddSlotException_Validation(t *testing.T) {
	cases := []struct {
		name string
		se   entity.SlotException
		err  error
	}{
		{
			"missing resource_id",
			entity.SlotException{
				Start: time.Unix(1, 0),
				End:   time.Unix(2, 0),
			},
			entity.ErrInvalidResourceID,
		},
		{
			"end before start",
			entity.SlotException{
				ResourceID: "res-1",
				Start:      time.Unix(2, 0),
				End:        time.Unix(1, 0),
			},
			entity.ErrInvalidTimeRange,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)

			c := New(
				storemocks.NewMockStore(ctrl),
				clientmocks.NewMockClient(ctrl),
				locationIQmocks.NewMockLocationIQClient(ctrl),
			)

			_, err := c.AddSlotException(context.Background(), tc.se)

			assert.ErrorIs(t, err, tc.err)
		})
	}
}

func TestSetLeavePeriod_Validation(t *testing.T) {
	ctrl := gomock.NewController(t)

	c := New(
		storemocks.NewMockStore(ctrl),
		clientmocks.NewMockClient(ctrl),
		locationIQmocks.NewMockLocationIQClient(ctrl),
	)

	_, err := c.SetLeavePeriod(
		context.Background(),
		entity.LeavePeriod{
			ResourceID: "res-1",
			Start:      time.Unix(2, 0),
			End:        time.Unix(1, 0),
		},
	)

	assert.ErrorIs(t, err, entity.ErrInvalidTimeRange)
}

func TestRemoveSlotException_CacheInvalidationIsBestEffort(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockStore := storemocks.NewMockStore(ctrl)
	mockClient := clientmocks.NewMockClient(ctrl)

	c := New(
		mockStore,
		mockClient,
		locationIQmocks.NewMockLocationIQClient(ctrl),
	)

	mockStore.EXPECT().
		BlockSlot(gomock.Any(), "slot-1", "unavailable").
		Return(entity.SlotStatusBlocked, nil)

	mockStore.EXPECT().
		GetSlot(gomock.Any(), "slot-1").
		Return(entity.Slot{}, errors.New("boom"))

	status, err := c.RemoveSlotException(
		context.Background(),
		"slot-1",
		"unavailable",
	)

	assert.NoError(t, err)
	assert.Equal(t, entity.SlotStatusBlocked, status)
}

func TestGetSlot_PassesThroughToStore(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockStore := storemocks.NewMockStore(ctrl)

	c := New(
		mockStore,
		clientmocks.NewMockClient(ctrl),
		locationIQmocks.NewMockLocationIQClient(ctrl),
	)

	mockStore.EXPECT().
		GetSlot(gomock.Any(), "slot-1").
		Return(entity.Slot{ID: "slot-1"}, nil)

	got, err := c.GetSlot(context.Background(), "slot-1")

	assert.NoError(t, err)
	assert.Equal(t, "slot-1", got.ID)
}

func TestSetResourceTypeStatus_InvalidatesGlobalCache(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockStore := storemocks.NewMockStore(ctrl)
	mockClient := clientmocks.NewMockClient(ctrl)

	c := New(
		mockStore,
		mockClient,
		locationIQmocks.NewMockLocationIQClient(ctrl),
	)

	mockStore.EXPECT().
		SetResourceTypeStatus(gomock.Any(), "type-1", false).
		Return(
			entity.ResourceType{
				ID:       "type-1",
				Name:     "doctor",
				IsActive: false,
			},
			nil,
		)

	mockClient.EXPECT().
		InvalidateGlobalSearchCache(gomock.Any()).
		Return(nil)

	_, err := c.SetResourceTypeStatus(
		context.Background(),
		"type-1",
		false,
	)

	assert.NoError(t, err)
}
