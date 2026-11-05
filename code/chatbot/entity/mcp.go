package entity

type Tool struct {
	Name        string
	Description string
	InputSchema map[string]any
}

type Resource struct {
	URI         string
	Name        string
	Description string
	MIMEType    string
}

type Prompt struct {
	Name        string
	Description string
	Arguments   []PromptArgument
}

type PromptArgument struct {
	Name        string
	Description string
	Required    bool
}
