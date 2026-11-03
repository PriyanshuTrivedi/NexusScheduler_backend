package handler

import (
	"context"

	"github.com/PriyanshuTrivedi/nexus-scheduler/code/chatbot/controller"
	"github.com/PriyanshuTrivedi/nexus-scheduler/code/chatbot/util"
	chatbotpb "github.com/PriyanshuTrivedi/nexus-scheduler/gen/idl/chatbot"
)

type Handler struct {
	chatbotpb.UnimplementedChatbotServiceServer
	ctl controller.Controller
}

func New(ctl controller.Controller) *Handler {
	return &Handler{
		ctl: ctl,
	}
}

func (h *Handler) SendMessage(ctx context.Context, req *chatbotpb.SendMessageRequest) (*chatbotpb.SendMessageResponse, error) {
	entityReq := util.SendMessageRequestFromProto(req)

	entityResp, err := h.ctl.SendMessage(ctx, entityReq)
	if err != nil {
		return nil, util.ErrorToGRPC(err)
	}

	return util.SendMessageResponseToProto(entityResp), nil
}

func (h *Handler) CloseConversation(ctx context.Context, req *chatbotpb.CloseConversationRequest) (*chatbotpb.CloseConversationResponse, error) {
	entityReq := util.CloseConversationRequestFromProto(req)

	entityResp, err := h.ctl.CloseConversation(ctx, entityReq)
	if err != nil {
		return nil, util.ErrorToGRPC(err)
	}

	return util.CloseConversationResponseToProto(entityResp), nil
}

func (h *Handler) ListConversations(ctx context.Context, req *chatbotpb.ListConversationsRequest) (*chatbotpb.ListConversationsResponse, error) {
	entityReq := util.ListConversationsRequestFromProto(req)

	entityResp, err := h.ctl.ListConversations(ctx, entityReq)
	if err != nil {
		return nil, util.ErrorToGRPC(err)
	}

	return util.ListConversationsResponseToProto(entityResp), nil
}

func (h *Handler) GetMessages(ctx context.Context, req *chatbotpb.GetMessagesRequest) (*chatbotpb.GetMessagesResponse, error) {
	entityReq := util.GetMessagesRequestFromProto(req)

	entityResp, err := h.ctl.GetMessages(ctx, entityReq)
	if err != nil {
		return nil, util.ErrorToGRPC(err)
	}

	return util.GetMessagesResponseToProto(entityResp), nil
}
