package util

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/PriyanshuTrivedi/nexus-scheduler/code/chatbot/entity"
	"github.com/PriyanshuTrivedi/nexus-scheduler/code/chatbot/store"
	chatbotpb "github.com/PriyanshuTrivedi/nexus-scheduler/gen/idl/chatbot"
	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestSendMessageRequestFromProto(t *testing.T) {
	tests := []struct {
		name string
		req  *chatbotpb.SendMessageRequest
		want *entity.SendMessageRequest
	}{
		{
			name: "nil request",
			req:  nil,
			want: nil,
		},
		{
			name: "valid request",
			req: &chatbotpb.SendMessageRequest{
				UserId:               "user-1",
				Message:              "Hello",
				ClientIdempotencyKey: "key-1",
			},
			want: &entity.SendMessageRequest{
				UserID:               "user-1",
				Message:              "Hello",
				ClientIdempotencyKey: "key-1",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := SendMessageRequestFromProto(tt.req)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestCloseConversationRequestFromProto(t *testing.T) {
	tests := []struct {
		name string
		req  *chatbotpb.CloseConversationRequest
		want *entity.CloseConversationRequest
	}{
		{
			name: "nil request",
			req:  nil,
			want: nil,
		},
		{
			name: "valid request",
			req: &chatbotpb.CloseConversationRequest{
				UserId: "user-1",
			},
			want: &entity.CloseConversationRequest{
				UserID: "user-1",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, CloseConversationRequestFromProto(tt.req))
		})
	}
}

func TestListConversationsRequestFromProto(t *testing.T) {
	tests := []struct {
		name string
		req  *chatbotpb.ListConversationsRequest
		want *entity.ListConversationsRequest
	}{
		{
			name: "nil request",
			req:  nil,
			want: nil,
		},
		{
			name: "valid request",
			req: &chatbotpb.ListConversationsRequest{
				UserId: "user-1",
				Limit:  20,
				Offset: 10,
			},
			want: &entity.ListConversationsRequest{
				UserID: "user-1",
				Limit:  20,
				Offset: 10,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, ListConversationsRequestFromProto(tt.req))
		})
	}
}

func TestGetMessagesRequestFromProto(t *testing.T) {
	tests := []struct {
		name string
		req  *chatbotpb.GetMessagesRequest
		want *entity.GetMessagesRequest
	}{
		{
			name: "nil request",
			req:  nil,
			want: nil,
		},
		{
			name: "valid request",
			req: &chatbotpb.GetMessagesRequest{
				UserId:         "user-1",
				ConversationId: "conversation-1",
			},
			want: &entity.GetMessagesRequest{
				UserID:         "user-1",
				ConversationID: "conversation-1",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, GetMessagesRequestFromProto(tt.req))
		})
	}
}

func TestValidateSendMessageRequest(t *testing.T) {
	tests := []struct {
		name    string
		req     *entity.SendMessageRequest
		wantErr error
	}{
		{
			name:    "nil request",
			req:     nil,
			wantErr: ErrInvalidRequest,
		},
		{
			name: "empty user ID",
			req: &entity.SendMessageRequest{
				Message: "Hello",
			},
			wantErr: ErrInvalidRequest,
		},
		{
			name: "whitespace user ID",
			req: &entity.SendMessageRequest{
				UserID:  "   ",
				Message: "Hello",
			},
			wantErr: ErrInvalidRequest,
		},
		{
			name: "empty message",
			req: &entity.SendMessageRequest{
				UserID: "user-1",
			},
			wantErr: ErrInvalidRequest,
		},
		{
			name: "whitespace message",
			req: &entity.SendMessageRequest{
				UserID:  "user-1",
				Message: "   ",
			},
			wantErr: ErrInvalidRequest,
		},
		{
			name: "valid request",
			req: &entity.SendMessageRequest{
				UserID:  "user-1",
				Message: "Hello",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateSendMessageRequest(tt.req)

			if tt.wantErr != nil {
				assert.ErrorIs(t, err, tt.wantErr)
				return
			}

			assert.NoError(t, err)
		})
	}
}

func TestValidateCloseConversationRequest(t *testing.T) {
	tests := []struct {
		name    string
		req     *entity.CloseConversationRequest
		wantErr error
	}{
		{
			name:    "nil request",
			req:     nil,
			wantErr: ErrInvalidRequest,
		},
		{
			name:    "empty user ID",
			req:     &entity.CloseConversationRequest{},
			wantErr: ErrInvalidRequest,
		},
		{
			name: "whitespace user ID",
			req: &entity.CloseConversationRequest{
				UserID: "   ",
			},
			wantErr: ErrInvalidRequest,
		},
		{
			name: "valid request",
			req: &entity.CloseConversationRequest{
				UserID: "user-1",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateCloseConversationRequest(tt.req)

			if tt.wantErr != nil {
				assert.ErrorIs(t, err, tt.wantErr)
				return
			}

			assert.NoError(t, err)
		})
	}
}

func TestValidateListConversationsRequest(t *testing.T) {
	tests := []struct {
		name    string
		req     *entity.ListConversationsRequest
		wantErr error
	}{
		{
			name:    "nil request",
			req:     nil,
			wantErr: ErrInvalidRequest,
		},
		{
			name:    "empty user ID",
			req:     &entity.ListConversationsRequest{},
			wantErr: ErrInvalidRequest,
		},
		{
			name: "negative offset",
			req: &entity.ListConversationsRequest{
				UserID: "user-1",
				Offset: -1,
			},
			wantErr: ErrInvalidRequest,
		},
		{
			name: "zero offset",
			req: &entity.ListConversationsRequest{
				UserID: "user-1",
				Offset: 0,
			},
		},
		{
			name: "valid request",
			req: &entity.ListConversationsRequest{
				UserID: "user-1",
				Limit:  20,
				Offset: 10,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateListConversationsRequest(tt.req)

			if tt.wantErr != nil {
				assert.ErrorIs(t, err, tt.wantErr)
				return
			}

			assert.NoError(t, err)
		})
	}
}

func TestValidateGetMessagesRequest(t *testing.T) {
	tests := []struct {
		name    string
		req     *entity.GetMessagesRequest
		wantErr error
	}{
		{
			name:    "nil request",
			req:     nil,
			wantErr: ErrInvalidRequest,
		},
		{
			name: "empty user ID",
			req: &entity.GetMessagesRequest{
				ConversationID: "conversation-1",
			},
			wantErr: ErrInvalidRequest,
		},
		{
			name: "empty conversation ID",
			req: &entity.GetMessagesRequest{
				UserID: "user-1",
			},
			wantErr: ErrInvalidRequest,
		},
		{
			name: "valid request",
			req: &entity.GetMessagesRequest{
				UserID:         "user-1",
				ConversationID: "conversation-1",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateGetMessagesRequest(tt.req)

			if tt.wantErr != nil {
				assert.ErrorIs(t, err, tt.wantErr)
				return
			}

			assert.NoError(t, err)
		})
	}
}

func TestMessageToProto(t *testing.T) {
	createdAt := time.Unix(1700000000, 0)

	tests := []struct {
		name string
		msg  *entity.Message
		want *chatbotpb.Message
	}{
		{
			name: "nil message",
			msg:  nil,
			want: nil,
		},
		{
			name: "user message",
			msg: &entity.Message{
				ID:        "message-1",
				Role:      entity.RoleUser,
				Content:   "Hello",
				CreatedAt: createdAt,
			},
			want: &chatbotpb.Message{
				MessageId:   "message-1",
				Role:        chatbotpb.Role_ROLE_USER,
				Content:     "Hello",
				CreatedUnix: createdAt.Unix(),
			},
		},
		{
			name: "assistant message",
			msg: &entity.Message{
				ID:        "message-2",
				Role:      entity.RoleAssistant,
				Content:   "Hi there",
				CreatedAt: createdAt,
			},
			want: &chatbotpb.Message{
				MessageId:   "message-2",
				Role:        chatbotpb.Role_ROLE_ASSISTANT,
				Content:     "Hi there",
				CreatedUnix: createdAt.Unix(),
			},
		},
		{
			name: "unknown role",
			msg: &entity.Message{
				ID:        "message-3",
				Role:      entity.Role("UNKNOWN"),
				Content:   "Unknown",
				CreatedAt: createdAt,
			},
			want: &chatbotpb.Message{
				MessageId:   "message-3",
				Role:        chatbotpb.Role(0),
				Content:     "Unknown",
				CreatedUnix: createdAt.Unix(),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, MessageToProto(tt.msg))
		})
	}
}

func TestConversationToProto(t *testing.T) {
	createdAt := time.Unix(1700000000, 0)
	updatedAt := time.Unix(1700000100, 0)

	userReason := entity.CloseReasonUser
	idleReason := entity.CloseReasonIdleTimeout

	tests := []struct {
		name         string
		conversation *entity.Conversation
		want         *chatbotpb.Conversation
	}{
		{
			name:         "nil conversation",
			conversation: nil,
			want:         nil,
		},
		{
			name: "active conversation",
			conversation: &entity.Conversation{
				ID:                 "conversation-1",
				Title:              "New chat",
				LastMessagePreview: "Hello",
				Status:             entity.StatusActive,
				CreatedAt:          createdAt,
				UpdatedAt:          updatedAt,
			},
			want: &chatbotpb.Conversation{
				ConversationId:     "conversation-1",
				Title:              "New chat",
				LastMessagePreview: "Hello",
				Status:             chatbotpb.ConversationStatus_CONVERSATION_STATUS_ACTIVE,
				CreatedUnix:        createdAt.Unix(),
				UpdatedUnix:        updatedAt.Unix(),
			},
		},
		{
			name: "closed by user",
			conversation: &entity.Conversation{
				ID:          "conversation-2",
				Status:      entity.StatusClosed,
				CloseReason: &userReason,
				CreatedAt:   createdAt,
				UpdatedAt:   updatedAt,
			},
			want: &chatbotpb.Conversation{
				ConversationId: "conversation-2",
				Status:         chatbotpb.ConversationStatus_CONVERSATION_STATUS_CLOSED,
				CloseReason:    ptr(chatbotpb.ConversationCloseReason_CLOSE_REASON_USER),
				CreatedUnix:    createdAt.Unix(),
				UpdatedUnix:    updatedAt.Unix(),
			},
		},
		{
			name: "closed due to idle timeout",
			conversation: &entity.Conversation{
				ID:          "conversation-3",
				Status:      entity.StatusClosed,
				CloseReason: &idleReason,
				CreatedAt:   createdAt,
				UpdatedAt:   updatedAt,
			},
			want: &chatbotpb.Conversation{
				ConversationId: "conversation-3",
				Status:         chatbotpb.ConversationStatus_CONVERSATION_STATUS_CLOSED,
				CloseReason:    ptr(chatbotpb.ConversationCloseReason_CLOSE_REASON_IDLE_TIMEOUT),
				CreatedUnix:    createdAt.Unix(),
				UpdatedUnix:    updatedAt.Unix(),
			},
		},
		{
			name: "unknown status and close reason",
			conversation: &entity.Conversation{
				ID:     "conversation-4",
				Status: entity.ConversationStatus("UNKNOWN"),
				CloseReason: func() *entity.CloseReason {
					reason := entity.CloseReason("UNKNOWN")
					return &reason
				}(),
				CreatedAt: createdAt,
				UpdatedAt: updatedAt,
			},
			want: &chatbotpb.Conversation{
				ConversationId: "conversation-4",
				Status:         chatbotpb.ConversationStatus(0),
				CloseReason:    ptr(chatbotpb.ConversationCloseReason(0)),
				CreatedUnix:    createdAt.Unix(),
				UpdatedUnix:    updatedAt.Unix(),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, ConversationToProto(tt.conversation))
		})
	}
}

func TestSendMessageResponseToProto(t *testing.T) {
	createdAt := time.Unix(1700000000, 0)

	tests := []struct {
		name string
		resp *entity.SendMessageResponse
		want *chatbotpb.SendMessageResponse
	}{
		{
			name: "nil response",
			resp: nil,
			want: nil,
		},
		{
			name: "valid response",
			resp: &entity.SendMessageResponse{
				Conversation: &entity.Conversation{
					ID: "conversation-1",
				},
				UserMessage: &entity.Message{
					ID:        "message-1",
					Role:      entity.RoleUser,
					Content:   "Hello",
					CreatedAt: createdAt,
				},
				AssistantMessage: &entity.Message{
					ID:        "message-2",
					Role:      entity.RoleAssistant,
					Content:   "Hi there",
					CreatedAt: createdAt,
				},
				IsNewConversation: true,
			},
			want: &chatbotpb.SendMessageResponse{
				ConversationId: "conversation-1",
				UserMessage: &chatbotpb.Message{
					MessageId:   "message-1",
					Role:        chatbotpb.Role_ROLE_USER,
					Content:     "Hello",
					CreatedUnix: createdAt.Unix(),
				},
				AssistantMessage: &chatbotpb.Message{
					MessageId:   "message-2",
					Role:        chatbotpb.Role_ROLE_ASSISTANT,
					Content:     "Hi there",
					CreatedUnix: createdAt.Unix(),
				},
				IsNewConversation: true,
			},
		},
		{
			name: "response without conversation",
			resp: &entity.SendMessageResponse{
				UserMessage: &entity.Message{
					ID:        "message-1",
					Role:      entity.RoleUser,
					Content:   "Hello",
					CreatedAt: createdAt,
				},
			},
			want: &chatbotpb.SendMessageResponse{
				UserMessage: &chatbotpb.Message{
					MessageId:   "message-1",
					Role:        chatbotpb.Role_ROLE_USER,
					Content:     "Hello",
					CreatedUnix: createdAt.Unix(),
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, SendMessageResponseToProto(tt.resp))
		})
	}
}

func TestCloseConversationResponseToProto(t *testing.T) {
	closedAt := time.Unix(1700000000, 0)

	tests := []struct {
		name string
		resp *entity.CloseConversationResponse
		want *chatbotpb.CloseConversationResponse
	}{
		{
			name: "nil response",
			resp: nil,
			want: nil,
		},
		{
			name: "closed",
			resp: &entity.CloseConversationResponse{
				ConversationID: "conversation-1",
				Closed:         true,
				ClosedAt:       closedAt,
			},
			want: &chatbotpb.CloseConversationResponse{
				ConversationId: "conversation-1",
				Closed:         true,
				ClosedUnix:     closedAt.Unix(),
			},
		},
		{
			name: "not closed",
			resp: &entity.CloseConversationResponse{
				Closed: false,
			},
			want: &chatbotpb.CloseConversationResponse{
				Closed: false,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, CloseConversationResponseToProto(tt.resp))
		})
	}
}

func TestListConversationsResponseToProto(t *testing.T) {
	createdAt := time.Unix(1700000000, 0)

	tests := []struct {
		name string
		resp *entity.ListConversationsResponse
		want *chatbotpb.ListConversationsResponse
	}{
		{
			name: "nil response",
			resp: nil,
			want: nil,
		},
		{
			name: "empty conversations",
			resp: &entity.ListConversationsResponse{},
			want: &chatbotpb.ListConversationsResponse{
				Conversations: []*chatbotpb.Conversation{},
			},
		},
		{
			name: "conversations present",
			resp: &entity.ListConversationsResponse{
				Conversations: []*entity.Conversation{
					{
						ID:        "conversation-1",
						Status:    entity.StatusActive,
						CreatedAt: createdAt,
						UpdatedAt: createdAt,
					},
				},
			},
			want: &chatbotpb.ListConversationsResponse{
				Conversations: []*chatbotpb.Conversation{
					{
						ConversationId: "conversation-1",
						Status:         chatbotpb.ConversationStatus_CONVERSATION_STATUS_ACTIVE,
						CreatedUnix:    createdAt.Unix(),
						UpdatedUnix:    createdAt.Unix(),
					},
				},
			},
		},
		{
			name: "nil conversation entry",
			resp: &entity.ListConversationsResponse{
				Conversations: []*entity.Conversation{nil},
			},
			want: &chatbotpb.ListConversationsResponse{
				Conversations: []*chatbotpb.Conversation{nil},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, ListConversationsResponseToProto(tt.resp))
		})
	}
}

func TestGetMessagesResponseToProto(t *testing.T) {
	createdAt := time.Unix(1700000000, 0)

	tests := []struct {
		name string
		resp *entity.GetMessagesResponse
		want *chatbotpb.GetMessagesResponse
	}{
		{
			name: "nil response",
			resp: nil,
			want: nil,
		},
		{
			name: "empty messages",
			resp: &entity.GetMessagesResponse{},
			want: &chatbotpb.GetMessagesResponse{
				Messages: []*chatbotpb.Message{},
			},
		},
		{
			name: "messages present",
			resp: &entity.GetMessagesResponse{
				Messages: []*entity.Message{
					{
						ID:        "message-1",
						Role:      entity.RoleUser,
						Content:   "Hello",
						CreatedAt: createdAt,
					},
				},
			},
			want: &chatbotpb.GetMessagesResponse{
				Messages: []*chatbotpb.Message{
					{
						MessageId:   "message-1",
						Role:        chatbotpb.Role_ROLE_USER,
						Content:     "Hello",
						CreatedUnix: createdAt.Unix(),
					},
				},
			},
		},
		{
			name: "nil message entry",
			resp: &entity.GetMessagesResponse{
				Messages: []*entity.Message{nil},
			},
			want: &chatbotpb.GetMessagesResponse{
				Messages: []*chatbotpb.Message{nil},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, GetMessagesResponseToProto(tt.resp))
		})
	}
}

func TestErrorToGRPC(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		wantCode codes.Code
		wantMsg  string
	}{
		{
			name: "nil error",
			err:  nil,
		},
		{
			name:     "invalid request",
			err:      ErrInvalidRequest,
			wantCode: codes.InvalidArgument,
			wantMsg:  ErrInvalidRequest.Error(),
		},
		{
			name:     "conversation not found",
			err:      store.ErrConversationNotFound,
			wantCode: codes.NotFound,
			wantMsg:  store.ErrConversationNotFound.Error(),
		},
		{
			name:     "conversation closed",
			err:      store.ErrConversationClosed,
			wantCode: codes.FailedPrecondition,
			wantMsg:  store.ErrConversationClosed.Error(),
		},
		{
			name:     "context cancelled",
			err:      context.Canceled,
			wantCode: codes.Canceled,
			wantMsg:  "request cancelled",
		},
		{
			name:     "context deadline exceeded",
			err:      context.DeadlineExceeded,
			wantCode: codes.DeadlineExceeded,
			wantMsg:  "request deadline exceeded",
		},
		{
			name:     "unknown error",
			err:      errors.New("database unavailable"),
			wantCode: codes.Internal,
			wantMsg:  "internal server error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ErrorToGRPC(tt.err)

			if tt.err == nil {
				assert.NoError(t, got)
				return
			}

			assert.Equal(t, tt.wantCode, status.Code(got))
			assert.Equal(t, tt.wantMsg, status.Convert(got).Message())
		})
	}
}

func TestErrorToGRPC_PreservesExistingGRPCError(t *testing.T) {
	original := status.Error(codes.PermissionDenied, "permission denied")

	got := ErrorToGRPC(original)

	assert.Equal(t, original, got)
	assert.Equal(t, codes.PermissionDenied, status.Code(got))
}

func ptr[T any](v T) *T {
	return &v
}
