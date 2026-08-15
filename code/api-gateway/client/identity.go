package client

import (
	"context"
	pb "github.com/PriyanshuTrivedi/nexus-scheduler/gen/idl/identity"
	"google.golang.org/grpc"
)

//go:generate mockgen -source=identity.go -destination=../../../gen/mocks/api_gateway/client/identity_mock.go -package=mocks

type IdentityClient interface {
	CreateOrganization(context.Context, *pb.CreateOrganizationRequest, ...grpc.CallOption) (*pb.CreateOrganizationResponse, error)
	UpdateOrganization(context.Context, *pb.UpdateOrganizationRequest, ...grpc.CallOption) (*pb.UpdateOrganizationResponse, error)
	GetOrganization(context.Context, *pb.GetOrganizationRequest, ...grpc.CallOption) (*pb.GetOrganizationResponse, error)
	ListOrganizations(context.Context, *pb.ListOrganizationsRequest, ...grpc.CallOption) (*pb.ListOrganizationsResponse, error)
	SetOrganizationStatus(context.Context, *pb.SetOrganizationStatusRequest, ...grpc.CallOption) (*pb.SetOrganizationStatusResponse, error)
	DeleteOrganization(context.Context, *pb.DeleteOrganizationRequest, ...grpc.CallOption) (*pb.DeleteOrganizationResponse, error)
	RegisterClient(context.Context, *pb.RegisterClientRequest, ...grpc.CallOption) (*pb.RegisterClientResponse, error)
	RegisterProvider(context.Context, *pb.RegisterProviderRequest, ...grpc.CallOption) (*pb.RegisterProviderResponse, error)
	Login(context.Context, *pb.LoginRequest, ...grpc.CallOption) (*pb.LoginResponse, error)
	UpdateProfile(context.Context, *pb.UpdateProfileRequest, ...grpc.CallOption) (*pb.UpdateProfileResponse, error)
	SetUserStatus(context.Context, *pb.SetUserStatusRequest, ...grpc.CallOption) (*pb.SetUserStatusResponse, error)
	GetUser(context.Context, *pb.GetUserRequest, ...grpc.CallOption) (*pb.GetUserResponse, error)
}
