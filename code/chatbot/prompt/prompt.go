package prompt

import _ "embed"

//go:embed system.md
var SystemInstruction string

//go:embed retrieval.md
var RetrievalInstruction string
