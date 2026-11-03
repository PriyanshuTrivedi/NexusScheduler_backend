package util

import (
	"errors"
	"fmt"
	"strings"

	"github.com/PriyanshuTrivedi/nexus-scheduler/code/chatbot/entity"
	"google.golang.org/genai"
)

var (
	ErrUnsupportedMessageRole = errors.New("unsupported message role")
	ErrEmptyResponse          = errors.New("empty response")
	ErrEmptyGeneratedReply    = errors.New("empty generated reply")
)

func MessagesToGeminiContents(history []entity.Message) ([]*genai.Content, error) {
	contents := make([]*genai.Content, 0, len(history))
	for _, msg := range history {
		var role genai.Role
		switch msg.Role {
		case entity.RoleUser:
			role = genai.RoleUser
		case entity.RoleAssistant:
			role = genai.RoleModel
		default:
			return nil, fmt.Errorf("%w: %s", ErrUnsupportedMessageRole, msg.Role)
		}

		contents = append(contents, genai.NewContentFromText(msg.Content, role))
	}
	return contents, nil
}

func GeminiResponseToText(response *genai.GenerateContentResponse) (string, error) {
	if response == nil {
		return "", ErrEmptyResponse
	}

	reply := strings.TrimSpace(response.Text())
	if reply == "" {
		return "", ErrEmptyGeneratedReply
	}

	return reply, nil
}
