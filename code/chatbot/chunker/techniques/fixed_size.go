package techniques

import (
	"fmt"
	"strings"

	"github.com/tiktoken-go/tokenizer"
)

//go:generate mockgen -source=fixed_size.go -destination=../../../../gen/mocks/chatbot/chunker/techniques/fixed_size_mock.go -package=mocks

type FixedSizeChunker interface {
	Chunk(text string) ([]string, error)
}

type FixedSizeChunkerConfig struct {
	ChunkSize    int
	Overlap      int
	MinChunkSize int
}

type fixedSizeChunker struct {
	config    FixedSizeChunkerConfig
	tokenizer tokenizer.Codec
}

func NewFixedSizeChunker(cfg FixedSizeChunkerConfig) (FixedSizeChunker, error) {
	enc, err := tokenizer.Get(tokenizer.Cl100kBase)
	if err != nil {
		return nil, fmt.Errorf("initialize tokenizer: %w", err)
	}

	return &fixedSizeChunker{
		config:    cfg,
		tokenizer: enc,
	}, nil
}

func (c *fixedSizeChunker) Chunk(text string) ([]string, error) {
	text = strings.TrimSpace(text)
	if text == "" {
		return nil, fmt.Errorf("cannot chunk empty text")
	}

	runes := []rune(text)
	chunks := make([]string, 0)

	for start := 0; start < len(runes); {
		end, err := c.findMaxEnd(runes, start)
		if err != nil {
			return nil, fmt.Errorf("find chunk boundary: %w", err)
		}

		if end <= start {
			return nil, fmt.Errorf("unable to make progress while chunking text")
		}

		if end < len(runes) {
			end = c.findPreferredBoundary(runes, start, end)
		}

		chunk := strings.TrimSpace(string(runes[start:end]))
		if chunk != "" {
			chunks = append(chunks, chunk)
		}

		if end >= len(runes) {
			break
		}

		nextStart, err := c.findOverlapStart(runes, start, end)
		if err != nil {
			return nil, fmt.Errorf("find overlap boundary: %w", err)
		}

		if nextStart <= start {
			return nil, fmt.Errorf("chunking failed to advance")
		}

		start = nextStart
	}

	return c.mergeSmallFinalChunk(chunks)
}

func (c *fixedSizeChunker) findMaxEnd(runes []rune, start int) (int, error) {
	low, high := start+1, len(runes)
	maxEnd := start

	for low <= high {
		mid := low + (high-low)/2

		count, err := c.tokenizer.Count(string(runes[start:mid]))
		if err != nil {
			return 0, err
		}

		if count <= c.config.ChunkSize {
			maxEnd = mid
			low = mid + 1
		} else {
			high = mid - 1
		}
	}

	return maxEnd, nil
}

func (c *fixedSizeChunker) findPreferredBoundary(runes []rune, start, end int) int {
	segment := string(runes[start:end])
	minOffset := len([]rune(segment)) * 3 / 4

	separators := []string{
		"\n\n",
		". ",
		"! ",
		"? ",
		"\n",
		" ",
	}

	for _, separator := range separators {
		index := strings.LastIndex(segment, separator)
		if index < 0 {
			continue
		}

		offset := len([]rune(segment[:index+len(separator)]))
		if offset >= minOffset {
			return start + offset
		}
	}

	return end
}

func (c *fixedSizeChunker) findOverlapStart(runes []rune, start, end int) (int, error) {
	if c.config.Overlap == 0 {
		return end, nil
	}

	low, high := start+1, end
	overlapStart := end

	for low <= high {
		mid := low + (high-low)/2

		count, err := c.tokenizer.Count(string(runes[mid:end]))
		if err != nil {
			return 0, err
		}

		if count <= c.config.Overlap {
			overlapStart = mid
			high = mid - 1
		} else {
			low = mid + 1
		}
	}

	for overlapStart < end && !IsWordBoundary(runes[overlapStart-1]) {
		overlapStart++
	}

	return overlapStart, nil
}

func (c *fixedSizeChunker) mergeSmallFinalChunk(chunks []string) ([]string, error) {
	if len(chunks) < 2 || c.config.MinChunkSize == 0 {
		return chunks, nil
	}

	lastIndex := len(chunks) - 1

	count, err := c.tokenizer.Count(chunks[lastIndex])
	if err != nil {
		return nil, fmt.Errorf("count final chunk tokens: %w", err)
	}

	if count >= c.config.MinChunkSize {
		return chunks, nil
	}

	merged := chunks[lastIndex-1] + "\n" + chunks[lastIndex]

	mergedCount, err := c.tokenizer.Count(merged)
	if err != nil {
		return nil, fmt.Errorf("count merged chunk tokens: %w", err)
	}

	if mergedCount <= c.config.ChunkSize {
		chunks[lastIndex-1] = merged
		return chunks[:lastIndex], nil
	}

	return chunks, nil
}

func IsWordBoundary(r rune) bool {
	return r == ' ' || r == '\n' || r == '\t' || r == '\r'
}
