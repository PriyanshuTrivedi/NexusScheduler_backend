package util

import (
	"testing"

	"github.com/PriyanshuTrivedi/nexus-scheduler/code/chatbot/entity"
	"github.com/stretchr/testify/assert"
	"google.golang.org/genai"
)

func TestMessagesToGeminiContents(t *testing.T) {
	tests := []struct {
		name     string
		messages []entity.Message
		want     []*genai.Content
		wantErr  error
	}{
		{
			name: "maps user and assistant messages",
			messages: []entity.Message{
				{Role: entity.RoleUser, Content: "Hello"},
				{Role: entity.RoleAssistant, Content: "Hi there"},
			},
			want: []*genai.Content{
				{
					Role:  genai.RoleUser,
					Parts: []*genai.Part{{Text: "Hello"}},
				},
				{
					Role:  genai.RoleModel,
					Parts: []*genai.Part{{Text: "Hi there"}},
				},
			},
		},
		{
			name: "returns empty contents for empty history",
		},
		{
			name: "rejects unsupported role",
			messages: []entity.Message{
				{Role: entity.Role("SYSTEM"), Content: "System prompt"},
			},
			wantErr: ErrUnsupportedMessageRole,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := MessagesToGeminiContents(tt.messages)

			if tt.wantErr != nil {
				assert.ErrorIs(t, err, tt.wantErr)
				assert.Nil(t, got)
				return
			}

			assert.NoError(t, err)
			if len(tt.messages) == 0 {
				assert.Empty(t, got)
			} else {
				assert.Equal(t, tt.want, got)
			}
		})
	}
}

func TestGeminiResponseToText(t *testing.T) {
	tests := []struct {
		name     string
		response *genai.GenerateContentResponse
		want     string
		wantErr  error
	}{
		{
			name:    "nil response",
			wantErr: ErrEmptyResponse,
		},
		{
			name: "returns generated text",
			response: &genai.GenerateContentResponse{
				Candidates: []*genai.Candidate{
					{
						Content: &genai.Content{
							Parts: []*genai.Part{
								{Text: "Hello"},
							},
						},
					},
				},
			},
			want: "Hello",
		},
		{
			name: "rejects empty generated text",
			response: &genai.GenerateContentResponse{
				Candidates: []*genai.Candidate{
					{
						Content: &genai.Content{
							Parts: []*genai.Part{
								{Text: "   "},
							},
						},
					},
				},
			},
			wantErr: ErrEmptyGeneratedReply,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := GeminiResponseToText(tt.response)

			if tt.wantErr != nil {
				assert.ErrorIs(t, err, tt.wantErr)
				assert.Empty(t, got)
				return
			}

			assert.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}
