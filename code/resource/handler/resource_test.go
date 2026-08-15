package handler

import (
	"context"
	"errors"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/PriyanshuTrivedi/nexus-scheduler/code/resource/entity"
	"github.com/PriyanshuTrivedi/nexus-scheduler/code/resource/store"
	pb "github.com/PriyanshuTrivedi/nexus-scheduler/gen/idl/resource"
	ctrlmocks "github.com/PriyanshuTrivedi/nexus-scheduler/gen/mocks/resource/controller"
)

func TestCreateResourceType_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockCtl := ctrlmocks.NewMockController(ctrl)
	h := New(mockCtl)
	mockCtl.EXPECT().CreateResourceType(gomock.Any(), "doctor").Return(entity.ResourceType{ID: "type-1", Name: "doctor"}, nil)
	resp, err := h.CreateResourceType(context.Background(), &pb.CreateResourceTypeRequest{Name: "doctor"})
	assert.NoError(t, err)
	assert.Equal(t, "type-1", resp.ResourceType.ResourceTypeId)
	assert.Equal(t, "doctor", resp.ResourceType.Name)
}

func TestDeleteResourceType_InUseMapsToFailedPrecondition(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockCtl := ctrlmocks.NewMockController(ctrl)
	h := New(mockCtl)
	mockCtl.EXPECT().DeleteResourceType(gomock.Any(), "type-1").Return(store.ErrResourceTypeInUse)
	_, err := h.DeleteResourceType(context.Background(), &pb.DeleteResourceTypeRequest{ResourceTypeId: "type-1"})
	assert.Equal(t, codes.FailedPrecondition, status.Code(err))
}

func TestCreateResource_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockCtl := ctrlmocks.NewMockController(ctrl)
	h := New(mockCtl)
	mockCtl.EXPECT().CreateResource(gomock.Any(), gomock.Any()).Return("res-1", nil)
	resp, err := h.CreateResource(context.Background(), &pb.CreateResourceRequest{TenantType: pb.TenantType_TENANT_TYPE_ORG, OrgId: "org-1", ResourceTypeId: "type-1", Name: "Dr. Rakesh", MeetingMode: pb.MeetingMode_MEETING_MODE_ONLINE})
	assert.NoError(t, err)
	assert.Equal(t, "res-1", resp.ResourceId)
}

func TestCreateResource_ValidationErrorMapsToInvalidArgument(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockCtl := ctrlmocks.NewMockController(ctrl)
	h := New(mockCtl)
	mockCtl.EXPECT().CreateResource(gomock.Any(), gomock.Any()).Return("", entity.ErrInvalidResourceTypeID)
	_, err := h.CreateResource(context.Background(), &pb.CreateResourceRequest{})
	assert.Equal(t, codes.InvalidArgument, status.Code(err))
}

func TestSetResourceStatus_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockCtl := ctrlmocks.NewMockController(ctrl)
	h := New(mockCtl)
	mockCtl.EXPECT().SetResourceStatus(gomock.Any(), "res-1", false).Return(nil)
	resp, err := h.SetResourceStatus(context.Background(), &pb.SetResourceStatusRequest{ResourceId: "res-1", IsActive: false})
	assert.NoError(t, err)
	assert.False(t, resp.IsActive)
}

func TestDeleteResource_ActiveResourceMapsToFailedPrecondition(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockCtl := ctrlmocks.NewMockController(ctrl)
	h := New(mockCtl)
	mockCtl.EXPECT().DeleteResource(gomock.Any(), "res-1").Return(store.ErrResourceMustBeInactive)
	_, err := h.DeleteResource(context.Background(), &pb.DeleteResourceRequest{ResourceId: "res-1"})
	assert.Equal(t, codes.FailedPrecondition, status.Code(err))
}

func TestCreateResource_UnknownErrorMapsToInternal(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockCtl := ctrlmocks.NewMockController(ctrl)
	h := New(mockCtl)
	mockCtl.EXPECT().CreateResource(gomock.Any(), gomock.Any()).Return("", errors.New("db is on fire"))
	_, err := h.CreateResource(context.Background(), &pb.CreateResourceRequest{})
	assert.Equal(t, codes.Internal, status.Code(err))
}

func TestGetSlot_NotFoundMapsToNotFound(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockCtl := ctrlmocks.NewMockController(ctrl)
	h := New(mockCtl)
	mockCtl.EXPECT().GetSlot(gomock.Any(), "missing").Return(entity.Slot{}, store.ErrSlotNotFound)
	_, err := h.GetSlot(context.Background(), &pb.GetSlotRequest{SlotId: "missing"})
	assert.Equal(t, codes.NotFound, status.Code(err))
}

func TestGetSlot_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockCtl := ctrlmocks.NewMockController(ctrl)
	h := New(mockCtl)
	mockCtl.EXPECT().GetSlot(gomock.Any(), "slot-1").Return(entity.Slot{ID: "slot-1", ResourceID: "res-1", Status: entity.SlotStatusOpen}, nil)
	resp, err := h.GetSlot(context.Background(), &pb.GetSlotRequest{SlotId: "slot-1"})
	assert.NoError(t, err)
	assert.Equal(t, "slot-1", resp.SlotId)
	assert.Equal(t, pb.SlotStatus_SLOT_STATUS_OPEN, resp.Status)
}

func TestAddSlotException_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockCtl := ctrlmocks.NewMockController(ctrl)
	h := New(mockCtl)
	mockCtl.EXPECT().AddSlotException(gomock.Any(), gomock.Any()).Return(entity.Slot{ID: "slot-1", Status: entity.SlotStatusOpen}, nil)
	resp, err := h.AddSlotException(context.Background(), &pb.AddSlotExceptionRequest{ResourceId: "res-1", StartUnix: 1000, EndUnix: 2000})
	assert.NoError(t, err)
	assert.Equal(t, "slot-1", resp.SlotId)
}

func TestSearchResources_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockCtl := ctrlmocks.NewMockController(ctrl)
	h := New(mockCtl)
	mockCtl.EXPECT().SearchResources(gomock.Any(), gomock.Any()).Return([]entity.ResourceSummary{{ResourceID: "res-1", Name: "Dr. Rakesh"}}, nil)
	resp, err := h.SearchResources(context.Background(), &pb.SearchResourcesRequest{OrgId: "org-1"})
	assert.NoError(t, err)
	assert.Len(t, resp.Resources, 1)
	assert.Equal(t, "res-1", resp.Resources[0].ResourceId)
}
