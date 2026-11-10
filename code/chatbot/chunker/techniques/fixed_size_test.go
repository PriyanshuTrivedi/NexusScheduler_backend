package techniques

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func newTestChunker(t *testing.T, cfg FixedSizeChunkerConfig) FixedSizeChunker {
	t.Helper()
	chunker, err := NewFixedSizeChunker(cfg)
	require.NoError(t, err)
	return chunker
}

func TestFixedSizeChunker_Chunk(t *testing.T) {
	t.Run("rejects empty text", func(t *testing.T) {
		chunker := newTestChunker(t, FixedSizeChunkerConfig{ChunkSize: 20, Overlap: 2})
		chunks, err := chunker.Chunk(" \n\t ")
		require.Error(t, err)
		require.Nil(t, chunks)
	})

	t.Run("trims input and returns one chunk when it fits", func(t *testing.T) {
		chunker := newTestChunker(t, FixedSizeChunkerConfig{ChunkSize: 100, Overlap: 10})
		chunks, err := chunker.Chunk("  a short document.  ")
		require.NoError(t, err)
		require.Equal(t, []string{"a short document."}, chunks)
	})

	t.Run("splits long text within token limit", func(t *testing.T) {
		chunker := newTestChunker(t, FixedSizeChunkerConfig{ChunkSize: 30, Overlap: 5, MinChunkSize: 0})
		text := strings.Repeat("The quick brown fox jumps over the lazy dog. ", 20)

		chunks, err := chunker.Chunk(text)
		require.NoError(t, err)
		require.Greater(t, len(chunks), 1)

		impl := chunker.(*fixedSizeChunker)
		for _, chunk := range chunks {
			count, countErr := impl.tokenizer.Count(chunk)
			require.NoError(t, countErr)
			require.LessOrEqual(t, count, 30)
			require.NotEmpty(t, chunk)
		}

		hasOverlap := false
		for i := 1; i < len(chunks); i++ {
			previousWords := strings.Fields(chunks[i-1])
			currentWords := strings.Fields(chunks[i])
			maxOverlap := min(len(previousWords), len(currentWords))
			for overlap := 1; overlap <= maxOverlap; overlap++ {
				previousSuffix := strings.Join(previousWords[len(previousWords)-overlap:], " ")
				currentPrefix := strings.Join(currentWords[:overlap], " ")
				if previousSuffix == currentPrefix {
					hasOverlap = true
					break
				}
			}
			if hasOverlap {
				break
			}
		}
		require.True(t, hasOverlap, "expected adjacent chunks to share overlapping text")
	})

	t.Run("supports zero overlap", func(t *testing.T) {
		chunker := newTestChunker(t, FixedSizeChunkerConfig{ChunkSize: 20, Overlap: 0})
		chunks, err := chunker.Chunk(strings.Repeat("word ", 40))
		require.NoError(t, err)
		require.Greater(t, len(chunks), 1)
	})

	t.Run("merges a small final chunk when combined size fits", func(t *testing.T) {
		impl := newTestChunker(t, FixedSizeChunkerConfig{ChunkSize: 20, Overlap: 0, MinChunkSize: 5}).(*fixedSizeChunker)
		chunks, err := impl.mergeSmallFinalChunk([]string{"first chunk", "tiny"})
		require.NoError(t, err)
		require.Equal(t, []string{"first chunk\ntiny"}, chunks)
	})

	t.Run("does not merge a small final chunk when combined size exceeds limit", func(t *testing.T) {
		impl := newTestChunker(t, FixedSizeChunkerConfig{ChunkSize: 3, Overlap: 0, MinChunkSize: 2}).(*fixedSizeChunker)
		chunks, err := impl.mergeSmallFinalChunk([]string{"one two three", "tiny"})
		require.NoError(t, err)
		require.Equal(t, []string{"one two three", "tiny"}, chunks)
	})

	t.Run("handles unicode text", func(t *testing.T) {
		chunker := newTestChunker(t, FixedSizeChunkerConfig{ChunkSize: 20, Overlap: 3})
		chunks, err := chunker.Chunk(strings.Repeat("नमस्ते 世界 🙂 ", 15))
		require.NoError(t, err)
		require.NotEmpty(t, chunks)
	})
}
