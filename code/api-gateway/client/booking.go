package client

import (
	"context"
	pb "github.com/PriyanshuTrivedi/nexus-scheduler/gen/idl/booking"
	"google.golang.org/grpc"
)

//go:generate mockgen -source=booking.go -destination=../../../gen/mocks/api_gateway/client/booking_mock.go -package=mocks

type BookingClient interface {
	CreateBooking(context.Context, *pb.CreateBookingRequest, ...grpc.CallOption) (*pb.CreateBookingResponse, error)
	CancelBooking(context.Context, *pb.CancelBookingRequest, ...grpc.CallOption) (*pb.CancelBookingResponse, error)
	RescheduleBooking(context.Context, *pb.RescheduleBookingRequest, ...grpc.CallOption) (*pb.RescheduleBookingResponse, error)
	GetBookingStatus(context.Context, *pb.GetBookingStatusRequest, ...grpc.CallOption) (*pb.GetBookingStatusResponse, error)
	ListUpcomingBookings(context.Context, *pb.ListUserBookingsRequest, ...grpc.CallOption) (*pb.ListUserBookingsResponse, error)
	ListPastBookings(context.Context, *pb.ListUserBookingsRequest, ...grpc.CallOption) (*pb.ListUserBookingsResponse, error)
}
