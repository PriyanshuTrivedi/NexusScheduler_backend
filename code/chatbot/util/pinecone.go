package util

import (
	"fmt"

	"github.com/PriyanshuTrivedi/nexus-scheduler/code/chatbot/entity"
)

func BuildPineconeVectors(objectID string, ownerUserID string, idempotencyKey string, embeddings []entity.Embedding) []entity.PineconeVector {
	vectors := make([]entity.PineconeVector, 0, len(embeddings))

	for i, embedding := range embeddings {
		vectors = append(vectors, entity.PineconeVector{
			ID:     fmt.Sprintf("%s_%d", objectID, i),
			Values: embedding.Vector,
			Metadata: map[string]interface{}{
				"object_id":       objectID,
				"owner_user_id":   ownerUserID,
				"chunk_index":     i,
				"content":         embedding.Content,
				"idempotency_key": idempotencyKey,
			},
		})
	}

	return vectors
}

func BuildPineconeDocumentFilter(objectID string, ownerUserID string) entity.PineconeFilter {
	return entity.PineconeFilter{
		"object_id": map[string]interface{}{
			"$eq": objectID,
		},
		"owner_user_id": map[string]interface{}{
			"$eq": ownerUserID,
		},
	}
}
