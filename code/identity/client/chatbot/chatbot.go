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
