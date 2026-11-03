package util

import (
	"context"
	"errors"
	"strings"

	"github.com/PriyanshuTrivedi/nexus-scheduler/code/chatbot/entity"
	"github.com/PriyanshuTrivedi/nexus-scheduler/code/chatbot/store"
	chatbotpb "github.com/PriyanshuTrivedi/nexus-scheduler/gen/idl/chatbot"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

var ErrInvalidRequest = errors.New("invalid request")

// Proto request -> Entity request

func SendMessageRequestFromProto(req *chatbotpb.SendMessageRequest) *entity.SendMessageRequest {
	if req == nil {
		return nil
	}

	return &entity.SendMessageRequest{
		UserID:               req.GetUserId(),
		Message:              req.GetMessage(),
		ClientIdempotencyKey: req.GetClientIdempotencyKey(),
	}
}

func CloseConversationRequestFromProto(req *chatbotpb.CloseConversationRequest) *entity.CloseConversationRequest {
	if req == nil {
		return nil
	}

	return &entity.CloseConversationRequest{
		UserID: req.GetUserId(),
	}
}

func ListConversationsRequestFromProto(req *chatbotpb.ListConversationsRequest) *entity.ListConversationsRequest {
	if req == nil {
		return nil
	}

	return &entity.ListConversationsRequest{
		UserID: req.GetUserId(),
		Limit:  req.GetLimit(),
		Offset: req.GetOffset(),
	}
}

func GetMessagesRequestFromProto(req *chatbotpb.GetMessagesRequest) *entity.GetMessagesRequest {
	if req == nil {
		return nil
	}

	return &entity.GetMessagesRequest{
		UserID:         req.GetUserId(),
		ConversationID: req.GetConversationId(),
	}
}

// Entity request validation

func ValidateSendMessageRequest(req *entity.SendMessageRequest) error {
	if req == nil ||
		strings.TrimSpace(req.UserID) == "" ||
		strings.TrimSpace(req.Message) == "" {
		return ErrInvalidRequest
	}
	return nil
}

func ValidateCloseConversationRequest(req *entity.CloseConversationRequest) error {
	if req == nil || strings.TrimSpace(req.UserID) == "" {
		return ErrInvalidRequest
	}
	return nil
}

func ValidateListConversationsRequest(req *entity.ListConversationsRequest) error {
	if req == nil ||
		strings.TrimSpace(req.UserID) == "" ||
		req.Offset < 0 {
		return ErrInvalidRequest
	}
	return nil
}

func ValidateGetMessagesRequest(req *entity.GetMessagesRequest) error {
	if req == nil ||
		strings.TrimSpace(req.UserID) == "" ||
		strings.TrimSpace(req.ConversationID) == "" {
		return ErrInvalidRequest
	}
	return nil
}

// Entity model -> Proto model

func MessageToProto(msg *entity.Message) *chatbotpb.Message {
	if msg == nil {
		return nil
	}

	var role chatbotpb.Role
	switch msg.Role {
	case entity.RoleUser:
		role = chatbotpb.Role_ROLE_USER
	case entity.RoleAssistant:
		role = chatbotpb.Role_ROLE_ASSISTANT
	}

	return &chatbotpb.Message{
		MessageId:   msg.ID,
		Role:        role,
		Content:     msg.Content,
		CreatedUnix: msg.CreatedAt.Unix(),
	}
}

func ConversationToProto(c *entity.Conversation) *chatbotpb.Conversation {
	if c == nil {
		return nil
	}

	var status chatbotpb.ConversationStatus
	switch c.Status {
	case entity.StatusActive:
		status = chatbotpb.ConversationStatus_CONVERSATION_STATUS_ACTIVE
	case entity.StatusClosed:
		status = chatbotpb.ConversationStatus_CONVERSATION_STATUS_CLOSED
	}

	var closeReason *chatbotpb.ConversationCloseReason
	if c.CloseReason != nil {
		var reason chatbotpb.ConversationCloseReason

		switch *c.CloseReason {
		case entity.CloseReasonUser:
			reason = chatbotpb.ConversationCloseReason_CLOSE_REASON_USER
		case entity.CloseReasonIdleTimeout:
			reason = chatbotpb.ConversationCloseReason_CLOSE_REASON_IDLE_TIMEOUT
		}

		closeReason = &reason
	}

	return &chatbotpb.Conversation{
		ConversationId:     c.ID,
		Title:              c.Title,
		LastMessagePreview: c.LastMessagePreview,
		Status:             status,
		CloseReason:        closeReason,
		CreatedUnix:        c.CreatedAt.Unix(),
		UpdatedUnix:        c.UpdatedAt.Unix(),
	}
}

// Entity response -> Proto response

func SendMessageResponseToProto(resp *entity.SendMessageResponse) *chatbotpb.SendMessageResponse {
	if resp == nil {
		return nil
	}

	var conversationID string
	if resp.Conversation != nil {
		conversationID = resp.Conversation.ID
	}

	return &chatbotpb.SendMessageResponse{
		ConversationId:    conversationID,
		UserMessage:       MessageToProto(resp.UserMessage),
		AssistantMessage:  MessageToProto(resp.AssistantMessage),
		IsNewConversation: resp.IsNewConversation,
	}
}

func CloseConversationResponseToProto(resp *entity.CloseConversationResponse) *chatbotpb.CloseConversationResponse {
	if resp == nil {
		return nil
	}

	response := &chatbotpb.CloseConversationResponse{
		Closed: resp.Closed,
	}

	if resp.Closed {
		response.ConversationId = resp.ConversationID
		response.ClosedUnix = resp.ClosedAt.Unix()
	}

	return response
}

func ListConversationsResponseToProto(resp *entity.ListConversationsResponse) *chatbotpb.ListConversationsResponse {
	if resp == nil {
		return nil
	}

	response := &chatbotpb.ListConversationsResponse{
		Conversations: make([]*chatbotpb.Conversation, 0, len(resp.Conversations)),
	}

	for _, conversation := range resp.Conversations {
		response.Conversations = append(response.Conversations, ConversationToProto(conversation))
	}

	return response
}

func GetMessagesResponseToProto(resp *entity.GetMessagesResponse) *chatbotpb.GetMessagesResponse {
	if resp == nil {
		return nil
	}

	response := &chatbotpb.GetMessagesResponse{
		Messages: make([]*chatbotpb.Message, 0, len(resp.Messages)),
	}

	for _, message := range resp.Messages {
		response.Messages = append(response.Messages, MessageToProto(message))
	}

	return response
}

// Error -> gRPC status

func ErrorToGRPC(err error) error {
	if err == nil {
		return nil
	}

	if _, ok := status.FromError(err); ok {
		return err
	}

	switch {
	case errors.Is(err, ErrInvalidRequest):
		return status.Error(codes.InvalidArgument, err.Error())
	case errors.Is(err, store.ErrConversationNotFound):
		return status.Error(codes.NotFound, err.Error())
	case errors.Is(err, store.ErrConversationClosed):
		return status.Error(codes.FailedPrecondition, err.Error())
	case errors.Is(err, context.Canceled):
		return status.Error(codes.Canceled, "request cancelled")
	case errors.Is(err, context.DeadlineExceeded):
		return status.Error(codes.DeadlineExceeded, "request deadline exceeded")
	default:
		return status.Error(codes.Internal, "internal server error")
	}
}
