package chatbot

import (
	"context"

	pb "github.com/PriyanshuTrivedi/nexus-scheduler/gen/idl/chatbot"
	"google.golang.org/grpc"
)

//go:generate mockgen -source=chatbot.go -destination=../../../../gen/mocks/identity/client/chatbot/chatbot_mock.go -package=mocks

type Client interface {
	IndexDocument(context.Context, *pb.IndexDocumentRequest, ...grpc.CallOption) (*pb.IndexDocumentResponse, error)
	DeleteDocumentIndex(context.Context, *pb.DeleteDocumentIndexRequest, ...grpc.CallOption) (*pb.DeleteDocumentIndexResponse, error)
}

type chatbotClient struct {
	client pb.ChatbotServiceClient
}

func New(conn *grpc.ClientConn) Client {
	return &chatbotClient{
		client: pb.NewChatbotServiceClient(conn),
	}
}

func (c *chatbotClient) IndexDocument(ctx context.Context, req *pb.IndexDocumentRequest, opts ...grpc.CallOption) (*pb.IndexDocumentResponse, error) {
	return c.client.IndexDocument(ctx, req, opts...)
}

func (c *chatbotClient) DeleteDocumentIndex(ctx context.Context, req *pb.DeleteDocumentIndexRequest, opts ...grpc.CallOption) (*pb.DeleteDocumentIndexResponse, error) {
	return c.client.DeleteDocumentIndex(ctx, req, opts...)
}
