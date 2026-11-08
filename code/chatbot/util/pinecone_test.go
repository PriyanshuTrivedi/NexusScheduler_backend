package util

import (
	"testing"

	"github.com/PriyanshuTrivedi/nexus-scheduler/code/chatbot/entity"
	"github.com/stretchr/testify/require"
)

func TestBuildPineconeVectors(t *testing.T) {
	embeddings := []entity.Embedding{
		{Vector: []float32{0.1, 0.2}, Content: "first chunk"},
		{Vector: []float32{0.3, 0.4}, Content: "second chunk"},
	}

	got := BuildPineconeVectors("object-1", "user-1", "key-1", embeddings)
	require.Len(t, got, 2)

	require.Equal(t, "object-1_0", got[0].ID)
	require.Equal(t, []float32{0.1, 0.2}, got[0].Values)
	require.Equal(t, map[string]interface{}{
		"object_id": "object-1", "owner_user_id": "user-1",
		"chunk_index": 0, "content": "first chunk", "idempotency_key": "key-1",
	}, got[0].Metadata)

	require.Equal(t, "object-1_1", got[1].ID)
	require.Equal(t, []float32{0.3, 0.4}, got[1].Values)
	require.Equal(t, "second chunk", got[1].Metadata["content"])
	require.Equal(t, 1, got[1].Metadata["chunk_index"])
}

func TestBuildPineconeVectors_EmptyEmbeddings(t *testing.T) {
	require.Empty(t, BuildPineconeVectors("object-1", "user-1", "key-1", nil))
}

func TestBuildPineconeDocumentFilter(t *testing.T) {
	require.Equal(t, entity.PineconeFilter{
		"object_id":     map[string]interface{}{"$eq": "object-1"},
		"owner_user_id": map[string]interface{}{"$eq": "user-1"},
	}, BuildPineconeDocumentFilter("object-1", "user-1"))
}
