package handler

import (
	"context"
	"testing"

	"github.com/PriyanshuTrivedi/nexus-scheduler/code/identity/entity"
	pb "github.com/PriyanshuTrivedi/nexus-scheduler/gen/idl/identity"
	controllermocks "github.com/PriyanshuTrivedi/nexus-scheduler/gen/mocks/identity/controller"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
)

func TestRegisterClient(t *testing.T) {
	ctrl := gomock.NewController(t)
	m := controllermocks.NewMockController(ctrl)
	h := New(m)
	m.EXPECT().RegisterClient(gomock.Any(), "Alice", entity.UserIdentifier{Email: "a@example.com"}, "pw").Return(entity.User{ID: "u-1", Role: entity.RoleClient}, nil)
	resp, err := h.RegisterClient(context.Background(), &pb.RegisterClientRequest{Name: "Alice", Identifier: &pb.UserIdentifier{Value: &pb.UserIdentifier_Email{Email: "a@example.com"}}, Password: "pw"})
	assert.NoError(t, err)
	assert.Equal(t, "u-1", resp.User.UserId)
}

func TestRegisterProvider(t *testing.T) {
	ctrl := gomock.NewController(t)
	m := controllermocks.NewMockController(ctrl)
	h := New(m)
	m.EXPECT().RegisterProvider(gomock.Any(), "Dr. Mehta", entity.UserIdentifier{Email: "doctor@example.com"}, "pw", entity.TenantTypeIndividual, "").Return(entity.User{ID: "r-1", Role: entity.RoleResource}, nil)
	resp, err := h.RegisterProvider(context.Background(), &pb.RegisterProviderRequest{Name: "Dr. Mehta", Identifier: &pb.UserIdentifier{Value: &pb.UserIdentifier_Email{Email: "doctor@example.com"}}, Password: "pw", TenantType: pb.TenantType_TENANT_TYPE_INDIVIDUAL})
	assert.NoError(t, err)
	assert.Equal(t, "r-1", resp.User.UserId)
}

func TestUpdateProfilePassesRequestedUserID(t *testing.T) {
	ctrl := gomock.NewController(t)
	m := controllermocks.NewMockController(ctrl)
	h := New(m)
	m.EXPECT().UpdateProfile(gomock.Any(), "u-1", "Alice").Return(entity.User{ID: "u-1", Name: "Alice", Role: entity.RoleClient}, nil)
	_, err := h.UpdateProfile(context.Background(), &pb.UpdateProfileRequest{UserId: "u-1", Name: "Alice"})
	assert.NoError(t, err)
}

func TestGetUserDoesNotPerformAuthorization(t *testing.T) {
	ctrl := gomock.NewController(t)
	m := controllermocks.NewMockController(ctrl)
	h := New(m)
	m.EXPECT().GetUser(gomock.Any(), "u-2").Return(entity.User{ID: "u-2", Role: entity.RoleClient}, nil)
	resp, err := h.GetUser(context.Background(), &pb.GetUserRequest{UserId: "u-2"})
	assert.NoError(t, err)
	assert.Equal(t, "u-2", resp.User.UserId)
}
