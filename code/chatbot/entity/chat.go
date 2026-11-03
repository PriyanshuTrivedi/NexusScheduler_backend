package entity

import "time"

type Role string

const (
	RoleUser      Role = "USER"
	RoleAssistant Role = "ASSISTANT"
)

type ConversationStatus string

const (
	StatusActive ConversationStatus = "ACTIVE"
	StatusClosed ConversationStatus = "CLOSED"
)

type CloseReason string

const (
	CloseReasonUser        CloseReason = "USER"
	CloseReasonIdleTimeout CloseReason = "IDLE_TIMEOUT"
)

type Message struct {
	ID        string
	Role      Role
	Content   string
	CreatedAt time.Time
}

type Conversation struct {
	ID                 string
	UserID             string
	Title              string
	LastMessagePreview string
	Status             ConversationStatus
	CloseReason        *CloseReason
	CreatedAt          time.Time
	UpdatedAt          time.Time
}

type SendMessageRequest struct {
	UserID               string
	Message              string
	ClientIdempotencyKey string
}

type SendMessageResponse struct {
	Conversation      *Conversation
	UserMessage       *Message
	AssistantMessage  *Message
	IsNewConversation bool
}

type CloseConversationRequest struct {
	UserID string
}

type CloseConversationResponse struct {
	ConversationID string
	Closed         bool
	ClosedAt       time.Time
}

type ListConversationsRequest struct {
	UserID string
	Limit  int32
	Offset int32
}

type ListConversationsResponse struct {
	Conversations []*Conversation
}

type GetMessagesRequest struct {
	UserID         string
	ConversationID string
}

type GetMessagesResponse struct {
	Messages []*Message
}
