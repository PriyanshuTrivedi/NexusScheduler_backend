package entity

type LLMMessage struct {
	Role              string
	Content           string
	FunctionCalls     []FunctionCall
	FunctionResponses []FunctionResponse
}

type LLMRequest struct {
	SystemInstruction string
	Messages          []LLMMessage
	Tools             []Tool
}

type LLMResponse struct {
	Text          string
	FunctionCalls []FunctionCall
}

type FunctionCall struct {
	ID               string
	Name             string
	Arguments        map[string]any
	ThoughtSignature []byte
}

type FunctionResponse struct {
	ID       string
	Name     string
	Response map[string]any
}

type Embedding struct {
	Vector  []float32
	Content string
}
