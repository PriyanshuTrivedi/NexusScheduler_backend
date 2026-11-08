package client

import (
	"context"
	"fmt"

	"github.com/PriyanshuTrivedi/nexus-scheduler/code/chatbot/entity"
	pineconesdk "github.com/pinecone-io/go-pinecone/v7/pinecone"
)

//go:generate mockgen -source=pinecone.go -destination=../../../gen/mocks/chatbot/client/pinecone_mock.go -package=mocks

const maxUpsertBatchSize = 1000

type PineconeClient interface {
	UpsertVectors(ctx context.Context, vectors []entity.PineconeVector) (uint32, error)
	DeleteVectorsByFilter(ctx context.Context, filter entity.PineconeFilter) error
}

type PineconeConfig struct {
	APIKey    string
	IndexHost string
}

type pineconeClient struct {
	index *pineconesdk.IndexConnection
}

func NewPineconeClient(cfg PineconeConfig) (PineconeClient, error) {
	client, err := pineconesdk.NewClient(pineconesdk.NewClientParams{
		ApiKey: cfg.APIKey,
	})
	if err != nil {
		return nil, fmt.Errorf("create Pinecone client: %w", err)
	}

	index, err := client.Index(pineconesdk.NewIndexConnParams{
		Host: cfg.IndexHost,
	})
	if err != nil {
		return nil, fmt.Errorf("connect to Pinecone index: %w", err)
	}

	return &pineconeClient{
		index: index,
	}, nil
}

func (c *pineconeClient) UpsertVectors(ctx context.Context, vectors []entity.PineconeVector) (uint32, error) {
	pineconeVectors := make([]*pineconesdk.Vector, 0, len(vectors))

	for _, vector := range vectors {
		metadata, err := pineconesdk.NewMetadata(vector.Metadata)
		if err != nil {
			return 0, fmt.Errorf("create metadata for vector %s: %w", vector.ID, err)
		}

		values := vector.Values

		pineconeVectors = append(pineconeVectors, &pineconesdk.Vector{
			Id:       vector.ID,
			Values:   &values,
			Metadata: metadata,
		})
	}

	var totalUpserted uint32

	for start := 0; start < len(pineconeVectors); start += maxUpsertBatchSize {
		end := min(start+maxUpsertBatchSize, len(pineconeVectors))

		upserted, err := c.index.UpsertVectors(ctx, pineconeVectors[start:end])
		if err != nil {
			return totalUpserted, fmt.Errorf(
				"upsert vectors batch starting at %d: %w",
				start,
				err,
			)
		}

		totalUpserted += upserted
	}

	return totalUpserted, nil
}

func (c *pineconeClient) DeleteVectorsByFilter(ctx context.Context, filter entity.PineconeFilter) error {
	pineconeFilter, err := pineconesdk.NewMetadataFilter(map[string]interface{}(filter))
	if err != nil {
		return fmt.Errorf("create Pinecone metadata filter: %w", err)
	}

	if err := c.index.DeleteVectorsByFilter(ctx, pineconeFilter); err != nil {
		return fmt.Errorf("delete vectors from Pinecone: %w", err)
	}

	return nil
}
