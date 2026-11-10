package client

import (
	"context"
	"testing"

	"github.com/PriyanshuTrivedi/nexus-scheduler/code/chatbot/entity"
	pineconesdk "github.com/pinecone-io/go-pinecone/v7/pinecone"
	"github.com/stretchr/testify/require"
)

func TestPineconeClient_UpsertVectors_EmptyInput(t *testing.T) {
	client := &pineconeClient{}

	count, err := client.UpsertVectors(context.Background(), nil)

	require.NoError(t, err)
	require.Zero(t, count)
}

func TestPineconeClient_UpsertVectors_EmptyID(t *testing.T) {
	client := &pineconeClient{}

	_, err := client.UpsertVectors(context.Background(), []entity.PineconeVector{
		{
			Values: []float32{0.1, 0.2},
		},
	})

	require.ErrorContains(t, err, "document ID is required")
}

func TestPineconeClient_UpsertVectors_EmptyEmbedding(t *testing.T) {
	client := &pineconeClient{}

	_, err := client.UpsertVectors(context.Background(), []entity.PineconeVector{
		{
			ID: "document-1",
		},
	})

	require.ErrorContains(t, err, "embedding is required")
}

func TestPineconeClient_UpsertVectors_InvalidMetadata(t *testing.T) {
	client := &pineconeClient{}

	_, err := client.UpsertVectors(context.Background(), []entity.PineconeVector{
		{
			ID:     "document-1",
			Values: []float32{0.1, 0.2},
			Metadata: map[string]interface{}{
				"unsupported": make(chan int),
			},
		},
	})

	require.Error(t, err)
	require.ErrorContains(t, err, "marshal Pinecone documents batch starting at 0")
}

func TestPineconeClient_DeleteVectorsByFilter_EmptyFilter(t *testing.T) {
	client := &pineconeClient{}

	err := client.DeleteVectorsByFilter(context.Background(), entity.PineconeFilter{})

	require.ErrorContains(t, err, "Pinecone delete filter cannot be empty")
}

func TestPineconeClient_DeleteVectorsByFilter_InvalidFilter(t *testing.T) {
	client := &pineconeClient{
		index: &pineconesdk.IndexConnection{},
	}

	err := client.DeleteVectorsByFilter(context.Background(), entity.PineconeFilter{
		"unsupported": make(chan int),
	})

	require.Error(t, err)
	require.ErrorContains(t, err, "delete documents from Pinecone")
	require.ErrorContains(t, err, "json: unsupported type: chan int")
}
