package client

import (
	"context"
	pb "github.com/PriyanshuTrivedi/nexus-scheduler/gen/idl/resource"
	"google.golang.org/grpc"
)

//go:generate mockgen -source=resource.go -destination=../../../gen/mocks/api_gateway/client/resource_mock.go -package=mocks

type ResourceClient interface {
	ListResourceTypes(context.Context, *pb.ListResourceTypesRequest, ...grpc.CallOption) (*pb.ListResourceTypesResponse, error)
	CreateResourceType(context.Context, *pb.CreateResourceTypeRequest, ...grpc.CallOption) (*pb.CreateResourceTypeResponse, error)
	SetResourceTypeStatus(context.Context, *pb.SetResourceTypeStatusRequest, ...grpc.CallOption) (*pb.SetResourceTypeStatusResponse, error)
	DeleteResourceType(context.Context, *pb.DeleteResourceTypeRequest, ...grpc.CallOption) (*pb.DeleteResourceTypeResponse, error)
	CreateResource(context.Context, *pb.CreateResourceRequest, ...grpc.CallOption) (*pb.CreateResourceResponse, error)
	SetResourceStatus(context.Context, *pb.SetResourceStatusRequest, ...grpc.CallOption) (*pb.SetResourceStatusResponse, error)
	DeleteResource(context.Context, *pb.DeleteResourceRequest, ...grpc.CallOption) (*pb.DeleteResourceResponse, error)
	SetRecurringAvailability(context.Context, *pb.SetRecurringAvailabilityRequest, ...grpc.CallOption) (*pb.SetRecurringAvailabilityResponse, error)
	AddSlotException(context.Context, *pb.AddSlotExceptionRequest, ...grpc.CallOption) (*pb.AddSlotExceptionResponse, error)
	RemoveSlotException(context.Context, *pb.RemoveSlotExceptionRequest, ...grpc.CallOption) (*pb.RemoveSlotExceptionResponse, error)
	SetLeavePeriod(context.Context, *pb.SetLeavePeriodRequest, ...grpc.CallOption) (*pb.SetLeavePeriodResponse, error)
	SearchResources(context.Context, *pb.SearchResourcesRequest, ...grpc.CallOption) (*pb.SearchResourcesResponse, error)
	GetSlot(context.Context, *pb.GetSlotRequest, ...grpc.CallOption) (*pb.GetSlotResponse, error)
}
