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
	storemocks "github.com/PriyanshuTrivedi/nexus-scheduler/gen/mocks/resource/store"
)

func validResource() entity.Resource {
	return entity.Resource{TenantType: entity.TenantTypeOrg, OrgID: "org-1", ResourceTypeID: "type-1", Name: "Dr. Rakesh", MeetingMode: entity.MeetingModeOnline}
}

func TestCreateResourceType_ValidationAndStore(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockStore := storemocks.NewMockStore(ctrl)
	c := New(mockStore, clientmocks.NewMockClient(ctrl))

	_, err := c.CreateResourceType(context.Background(), "")
	assert.ErrorIs(t, err, entity.ErrInvalidResourceTypeName)

	mockStore.EXPECT().CreateResourceType(gomock.Any(), "doctor").Return(entity.ResourceType{ID: "type-1", Name: "doctor"}, nil)
	got, err := c.CreateResourceType(context.Background(), "doctor")
	assert.NoError(t, err)
	assert.Equal(t, "type-1", got.ID)
}

func TestDeleteResourceType_Validation(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockStore := storemocks.NewMockStore(ctrl)
	c := New(mockStore, clientmocks.NewMockClient(ctrl))

	assert.ErrorIs(t, c.DeleteResourceType(context.Background(), ""), entity.ErrInvalidResourceTypeID)
	mockStore.EXPECT().DeleteResourceType(gomock.Any(), "type-1").Return(nil)
	assert.NoError(t, c.DeleteResourceType(context.Background(), "type-1"))
}

func TestCreateResource_ExpandsRecurrenceAndInvalidatesCache(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockStore := storemocks.NewMockStore(ctrl)
	mockClient := clientmocks.NewMockClient(ctrl)
	c := New(mockStore, mockClient)

	r := validResource()
	r.Recurrence = []entity.RecurrenceRule{{Day: entity.Monday, Timezone: "Asia/Kolkata", Slots: []entity.TimeSlot{{StartHour: 9, EndHour: 17}}}}
	mockStore.EXPECT().CreateResource(gomock.Any(), gomock.Any(), gomock.Not(gomock.Nil())).DoAndReturn(func(_ context.Context, _ entity.Resource, slots []entity.Slot) (string, error) {
		assert.NotEmpty(t, slots)
		return "res-1", nil
	})
	mockClient.EXPECT().InvalidateOrgSearchCache(gomock.Any(), "org-1").Return(nil)

	id, err := c.CreateResource(context.Background(), r)
	assert.NoError(t, err)
	assert.Equal(t, "res-1", id)
}

func TestSetResourceStatus_InvalidResourceID(t *testing.T) {
	ctrl := gomock.NewController(t)
	c := New(storemocks.NewMockStore(ctrl), clientmocks.NewMockClient(ctrl))
	assert.ErrorIs(t, c.SetResourceStatus(context.Background(), "", false), entity.ErrInvalidResourceID)
}

func TestSetResourceStatus_InvalidatesOrgCache(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockStore := storemocks.NewMockStore(ctrl)
	mockClient := clientmocks.NewMockClient(ctrl)
	c := New(mockStore, mockClient)
	mockStore.EXPECT().SetResourceStatus(gomock.Any(), "res-1", false).Return(func() *string { v := "org-1"; return &v }(), nil)
	mockClient.EXPECT().InvalidateOrgSearchCache(gomock.Any(), "org-1").Return(nil)
	assert.NoError(t, c.SetResourceStatus(context.Background(), "res-1", false))
}

func TestDeleteResource_OnlyDelegatesAndInvalidates(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockStore := storemocks.NewMockStore(ctrl)
	mockClient := clientmocks.NewMockClient(ctrl)
	c := New(mockStore, mockClient)
	mockStore.EXPECT().DeleteResource(gomock.Any(), "res-1").Return(func() *string { v := "org-1"; return &v }(), nil)
	mockClient.EXPECT().InvalidateOrgSearchCache(gomock.Any(), "org-1").Return(nil)
	assert.NoError(t, c.DeleteResource(context.Background(), "res-1"))
}

func TestDeleteResource_Validation(t *testing.T) {
	ctrl := gomock.NewController(t)
	c := New(storemocks.NewMockStore(ctrl), clientmocks.NewMockClient(ctrl))
	assert.ErrorIs(t, c.DeleteResource(context.Background(), ""), entity.ErrInvalidResourceID)
}

func TestExpandRecurrence_NeverProducesPastSlots(t *testing.T) {
	ctrl := gomock.NewController(t)
	c := New(storemocks.NewMockStore(ctrl), clientmocks.NewMockClient(ctrl)).(*resourceController)
	loc, _ := time.LoadLocation("Asia/Kolkata")
	yesterday := time.Now().In(loc).AddDate(0, 0, -1)
	rules := []entity.RecurrenceRule{{Day: entity.DayOfWeek(int(yesterday.Weekday()) + 1), Timezone: "Asia/Kolkata", Slots: []entity.TimeSlot{{StartHour: 0, EndHour: 1}}}}
	slots, err := c.expandRecurrence(rules, yesterday)
	assert.NoError(t, err)
	for _, s := range slots {
		assert.False(t, s.End.Before(time.Now()))
	}
}

func TestExpandRecurrence_BadTimezoneErrors(t *testing.T) {
	ctrl := gomock.NewController(t)
	c := New(storemocks.NewMockStore(ctrl), clientmocks.NewMockClient(ctrl)).(*resourceController)
	rules := []entity.RecurrenceRule{{Day: entity.Monday, Timezone: "Not/AZone", Slots: []entity.TimeSlot{{StartHour: 9, EndHour: 17}}}}
	_, err := c.expandRecurrence(rules, time.Now())
	assert.Error(t, err)
}

func TestAddSlotException_Validation(t *testing.T) {
	cases := []struct {
		name string
		se   entity.SlotException
		err  error
	}{
		{"missing resource_id", entity.SlotException{Start: time.Unix(1, 0), End: time.Unix(2, 0)}, entity.ErrInvalidResourceID},
		{"end before start", entity.SlotException{ResourceID: "res-1", Start: time.Unix(2, 0), End: time.Unix(1, 0)}, entity.ErrInvalidTimeRange},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			c := New(storemocks.NewMockStore(ctrl), clientmocks.NewMockClient(ctrl))
			_, err := c.AddSlotException(context.Background(), tc.se)
			assert.ErrorIs(t, err, tc.err)
		})
	}
}

func TestSetLeavePeriod_Validation(t *testing.T) {
	ctrl := gomock.NewController(t)
	c := New(storemocks.NewMockStore(ctrl), clientmocks.NewMockClient(ctrl))
	_, err := c.SetLeavePeriod(context.Background(), entity.LeavePeriod{ResourceID: "res-1", Start: time.Unix(2, 0), End: time.Unix(1, 0)})
	assert.ErrorIs(t, err, entity.ErrInvalidTimeRange)
}

func TestRemoveSlotException_CacheInvalidationIsBestEffort(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockStore := storemocks.NewMockStore(ctrl)
	mockClient := clientmocks.NewMockClient(ctrl)
	c := New(mockStore, mockClient)
	mockStore.EXPECT().BlockSlot(gomock.Any(), "slot-1", "unavailable").Return(entity.SlotStatusBlocked, nil)
	mockStore.EXPECT().GetSlot(gomock.Any(), "slot-1").Return(entity.Slot{}, errors.New("boom"))
	status, err := c.RemoveSlotException(context.Background(), "slot-1", "unavailable")
	assert.NoError(t, err)
	assert.Equal(t, entity.SlotStatusBlocked, status)
}

func TestGetSlot_PassesThroughToStore(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockStore := storemocks.NewMockStore(ctrl)
	c := New(mockStore, clientmocks.NewMockClient(ctrl))
	mockStore.EXPECT().GetSlot(gomock.Any(), "slot-1").Return(entity.Slot{ID: "slot-1"}, nil)
	got, err := c.GetSlot(context.Background(), "slot-1")
	assert.NoError(t, err)
	assert.Equal(t, "slot-1", got.ID)
}

func TestSetResourceTypeStatus_InvalidatesGlobalCache(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockStore := storemocks.NewMockStore(ctrl)
	mockClient := clientmocks.NewMockClient(ctrl)
	c := New(mockStore, mockClient)
	mockStore.EXPECT().SetResourceTypeStatus(gomock.Any(), "type-1", false).Return(entity.ResourceType{ID: "type-1", Name: "doctor", IsActive: false}, nil)
	mockClient.EXPECT().InvalidateGlobalSearchCache(gomock.Any()).Return(nil)
	_, err := c.SetResourceTypeStatus(context.Background(), "type-1", false)
	assert.NoError(t, err)
}
