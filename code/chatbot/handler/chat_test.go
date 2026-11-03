package handler

import (
	"context"
	"testing"
	"time"

	"github.com/PriyanshuTrivedi/nexus-scheduler/code/chatbot/entity"
	chatbotpb "github.com/PriyanshuTrivedi/nexus-scheduler/gen/idl/chatbot"
	ctrlmocks "github.com/PriyanshuTrivedi/nexus-scheduler/gen/mocks/chatbot/controller"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestSendMessage(t *testing.T) {
	now := time.Date(2026, 11, 4, 10, 0, 0, 0, time.UTC)

	tests := []struct {
		name    string
		resp    *entity.SendMessageResponse
		err     error
		wantErr codes.Code
	}{
		{
			name: "success",
			resp: &entity.SendMessageResponse{
				Conversation: &entity.Conversation{
					ID:        "conversation-1",
					Title:     "New chat",
					Status:    entity.StatusActive,
					CreatedAt: now,
					UpdatedAt: now,
				},
				UserMessage: &entity.Message{
					ID:        "message-1",
					Role:      entity.RoleUser,
					Content:   "Hello",
					CreatedAt: now,
				},
				AssistantMessage: &entity.Message{
					ID:        "message-2",
					Role:      entity.RoleAssistant,
					Content:   "Hello! How can I help?",
					CreatedAt: now,
				},
				IsNewConversation: true,
			},
		},
		{
			name:    "controller error",
			err:     assert.AnError,
			wantErr: codes.Internal,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			mockCtl := ctrlmocks.NewMockController(ctrl)
			h := New(mockCtl)

			mockCtl.EXPECT().
				SendMessage(gomock.Any(), gomock.Any()).
				DoAndReturn(func(ctx context.Context, req *entity.SendMessageRequest) (*entity.SendMessageResponse, error) {
					assert.Equal(t, "user-1", req.UserID)
					assert.Equal(t, "Hello", req.Message)
					return tt.resp, tt.err
				})

			resp, err := h.SendMessage(context.Background(), &chatbotpb.SendMessageRequest{
				UserId:  "user-1",
				Message: "Hello",
			})

			if tt.wantErr != codes.OK {
				assert.Nil(t, resp)
				assert.Equal(t, tt.wantErr, status.Code(err))
				return
			}

			assert.NoError(t, err)
			if !assert.NotNil(t, resp) {
				return
			}

			assert.Equal(t, "conversation-1", resp.ConversationId)
			assert.Equal(t, "message-1", resp.UserMessage.MessageId)
			assert.Equal(t, "message-2", resp.AssistantMessage.MessageId)
			assert.Equal(t, "Hello", resp.UserMessage.Content)
			assert.Equal(t, "Hello! How can I help?", resp.AssistantMessage.Content)
			assert.True(t, resp.IsNewConversation)
		})
	}
}

func TestCloseConversation(t *testing.T) {
	now := time.Date(2026, 11, 4, 10, 0, 0, 0, time.UTC)

	tests := []struct {
		name    string
		resp    *entity.CloseConversationResponse
		err     error
		wantErr codes.Code
	}{
		{
			name: "success",
			resp: &entity.CloseConversationResponse{
				ConversationID: "conversation-1",
				Closed:         true,
				ClosedAt:       now,
			},
		},
		{
			name:    "controller error",
			err:     assert.AnError,
			wantErr: codes.Internal,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			mockCtl := ctrlmocks.NewMockController(ctrl)
			h := New(mockCtl)

			mockCtl.EXPECT().
				CloseConversation(gomock.Any(), gomock.Any()).
				DoAndReturn(func(ctx context.Context, req *entity.CloseConversationRequest) (*entity.CloseConversationResponse, error) {
					assert.Equal(t, "user-1", req.UserID)
					return tt.resp, tt.err
				})

			resp, err := h.CloseConversation(context.Background(), &chatbotpb.CloseConversationRequest{
				UserId: "user-1",
			})

			if tt.wantErr != codes.OK {
				assert.Nil(t, resp)
				assert.Equal(t, tt.wantErr, status.Code(err))
				return
			}

			assert.NoError(t, err)
			if !assert.NotNil(t, resp) {
				return
			}

			assert.True(t, resp.Closed)
			assert.Equal(t, "conversation-1", resp.ConversationId)
		})
	}
}

func TestListConversations(t *testing.T) {
	now := time.Date(2026, 11, 4, 10, 0, 0, 0, time.UTC)

	tests := []struct {
		name    string
		resp    *entity.ListConversationsResponse
		err     error
		wantErr codes.Code
	}{
		{
			name: "success",
			resp: &entity.ListConversationsResponse{
				Conversations: []*entity.Conversation{
					{
						ID:        "conversation-1",
						Title:     "First chat",
						Status:    entity.StatusActive,
						CreatedAt: now,
						UpdatedAt: now,
					},
					{
						ID:        "conversation-2",
						Title:     "Second chat",
						Status:    entity.StatusClosed,
						CreatedAt: now,
						UpdatedAt: now,
					},
				},
			},
		},
		{
			name:    "controller error",
			err:     assert.AnError,
			wantErr: codes.Internal,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			mockCtl := ctrlmocks.NewMockController(ctrl)
			h := New(mockCtl)

			mockCtl.EXPECT().
				ListConversations(gomock.Any(), gomock.Any()).
				DoAndReturn(func(ctx context.Context, req *entity.ListConversationsRequest) (*entity.ListConversationsResponse, error) {
					assert.Equal(t, "user-1", req.UserID)
					assert.Equal(t, int32(10), req.Limit)
					assert.Equal(t, int32(5), req.Offset)
					return tt.resp, tt.err
				})

			resp, err := h.ListConversations(context.Background(), &chatbotpb.ListConversationsRequest{
				UserId: "user-1",
				Limit:  10,
				Offset: 5,
			})

			if tt.wantErr != codes.OK {
				assert.Nil(t, resp)
				assert.Equal(t, tt.wantErr, status.Code(err))
				return
			}

			assert.NoError(t, err)
			if !assert.NotNil(t, resp) {
				return
			}

			assert.Len(t, resp.Conversations, 2)
			assert.Equal(t, "conversation-1", resp.Conversations[0].ConversationId)
			assert.Equal(t, "First chat", resp.Conversations[0].Title)
			assert.Equal(t, "conversation-2", resp.Conversations[1].ConversationId)
		})
	}
}

func TestGetMessages(t *testing.T) {
	now := time.Date(2026, 11, 4, 10, 0, 0, 0, time.UTC)

	tests := []struct {
		name    string
		resp    *entity.GetMessagesResponse
		err     error
		wantErr codes.Code
	}{
		{
			name: "success",
			resp: &entity.GetMessagesResponse{
				Messages: []*entity.Message{
					{
						ID:        "message-1",
						Role:      entity.RoleUser,
						Content:   "Hello",
						CreatedAt: now,
					},
					{
						ID:        "message-2",
						Role:      entity.RoleAssistant,
						Content:   "Hi there!",
						CreatedAt: now,
					},
				},
			},
		},
		{
			name:    "controller error",
			err:     assert.AnError,
			wantErr: codes.Internal,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			mockCtl := ctrlmocks.NewMockController(ctrl)
			h := New(mockCtl)

			mockCtl.EXPECT().
				GetMessages(gomock.Any(), gomock.Any()).
				DoAndReturn(func(ctx context.Context, req *entity.GetMessagesRequest) (*entity.GetMessagesResponse, error) {
					assert.Equal(t, "user-1", req.UserID)
					assert.Equal(t, "conversation-1", req.ConversationID)
					return tt.resp, tt.err
				})

			resp, err := h.GetMessages(context.Background(), &chatbotpb.GetMessagesRequest{
				UserId:         "user-1",
				ConversationId: "conversation-1",
			})

			if tt.wantErr != codes.OK {
				assert.Nil(t, resp)
				assert.Equal(t, tt.wantErr, status.Code(err))
				return
			}

			assert.NoError(t, err)
			if !assert.NotNil(t, resp) {
				return
			}

			assert.Len(t, resp.Messages, 2)
			assert.Equal(t, "message-1", resp.Messages[0].MessageId)
			assert.Equal(t, "Hello", resp.Messages[0].Content)
			assert.Equal(t, "message-2", resp.Messages[1].MessageId)
			assert.Equal(t, "Hi there!", resp.Messages[1].Content)
		})
	}
}
