package chunker

//go:generate mockgen -source=chunker.go -destination=../../../gen/mocks/chatbot/chunker/chunker_mock.go -package=mocks

type Chunker interface {
	Chunk(text string) ([]string, error)
}
