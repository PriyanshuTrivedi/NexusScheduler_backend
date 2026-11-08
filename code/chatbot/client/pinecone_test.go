package client

import (
	"context"
	"testing"

	"github.com/PriyanshuTrivedi/nexus-scheduler/code/chatbot/entity"
	"github.com/stretchr/testify/require"
)

func TestPineconeClient_UpsertVectors_EmptyInput(t *testing.T) {
	client := &pineconeClient{}

	count, err := client.UpsertVectors(context.Background(), nil)
	require.NoError(t, err)
	require.Zero(t, count)
}

func TestPineconeClient_UpsertVectors_InvalidMetadata(t *testing.T) {
	client := &pineconeClient{}

	_, err := client.UpsertVectors(context.Background(), []entity.PineconeVector{
		{
			ID:     "vector-1",
			Values: []float32{0.1, 0.2},
			Metadata: map[string]interface{}{
				"unsupported": make(chan int),
			},
		},
	})

	require.Error(t, err)
	require.ErrorContains(t, err, "create metadata for vector vector-1")
}

func TestPineconeClient_DeleteVectorsByFilter_InvalidFilter(t *testing.T) {
	client := &pineconeClient{}

	err := client.DeleteVectorsByFilter(context.Background(), entity.PineconeFilter{
		"unsupported": make(chan int),
	})

	require.Error(t, err)
	require.ErrorContains(t, err, "create Pinecone metadata filter")
}
