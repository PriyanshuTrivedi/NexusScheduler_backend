package client

import (
	"context"

	pb "github.com/PriyanshuTrivedi/nexus-scheduler/gen/idl/chatbot"
	"google.golang.org/grpc"
)

//go:generate mockgen -source=chatbot.go -destination=../../../gen/mocks/api_gateway/client/chatbot_mock.go -package=mocks

type ChatbotClient interface {
	SendMessage(context.Context, *pb.SendMessageRequest, ...grpc.CallOption) (*pb.SendMessageResponse, error)
	CloseConversation(context.Context, *pb.CloseConversationRequest, ...grpc.CallOption) (*pb.CloseConversationResponse, error)
	ListConversations(context.Context, *pb.ListConversationsRequest, ...grpc.CallOption) (*pb.ListConversationsResponse, error)
	GetMessages(context.Context, *pb.GetMessagesRequest, ...grpc.CallOption) (*pb.GetMessagesResponse, error)
}
