package client

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/PriyanshuTrivedi/nexus-scheduler/code/chatbot/entity"
	pineconesdk "github.com/pinecone-io/go-pinecone/v7/pinecone"
)

//go:generate mockgen -source=pinecone.go -destination=../../../gen/mocks/chatbot/client/pinecone_mock.go -package=mocks

const (
	maxUpsertBatchSize  = 1000
	maxUpsertBatchBytes = 2 * 1024 * 1024
)

type PineconeClient interface {
	UpsertVectors(ctx context.Context, vectors []entity.PineconeVector) (uint32, error)
	DeleteVectorsByFilter(ctx context.Context, filter entity.PineconeFilter) error
	SearchVectors(ctx context.Context, vector []float32) ([]string, error)
}

type PineconeConfig struct {
	APIKey    string
	IndexHost string
	Namespace string
	TopK      int32
}

type pineconeClient struct {
	index *pineconesdk.IndexConnection
	topK  int32
}

func NewPineconeClient(cfg PineconeConfig) (PineconeClient, error) {
	client, err := pineconesdk.NewClient(pineconesdk.NewClientParams{
		ApiKey: cfg.APIKey,
	})
	if err != nil {
		return nil, fmt.Errorf("create Pinecone client: %w", err)
	}

	index, err := client.Index(pineconesdk.NewIndexConnParams{
		Host:      cfg.IndexHost,
		Namespace: cfg.Namespace,
	})
	if err != nil {
		return nil, fmt.Errorf("connect to Pinecone index: %w", err)
	}

	return &pineconeClient{
		index: index,
		topK:  cfg.TopK,
	}, nil
}

func (c *pineconeClient) UpsertVectors(ctx context.Context, vectors []entity.PineconeVector) (uint32, error) {
	if len(vectors) == 0 {
		return 0, nil
	}

	documents := make([]pineconesdk.Document, 0, len(vectors))

	for _, vector := range vectors {
		if vector.ID == "" {
			return 0, fmt.Errorf("create Pinecone document: document ID is required")
		}
		if len(vector.Values) == 0 {
			return 0, fmt.Errorf("create Pinecone document %s: embedding is required", vector.ID)
		}

		document := pineconesdk.Document{
			"_id":       vector.ID,
			"embedding": vector.Values,
		}

		for key, value := range vector.Metadata {
			if key == "" || key == "_id" || key == "embedding" {
				continue
			}

			document[key] = value
		}

		documents = append(documents, document)
	}

	var totalUpserted uint32

	for start := 0; start < len(documents); {
		end := start
		batch := make([]pineconesdk.Document, 0)

		for end < len(documents) && len(batch) < maxUpsertBatchSize {
			candidate := append(batch, documents[end])

			payload, err := json.Marshal(candidate)
			if err != nil {
				return totalUpserted, fmt.Errorf("marshal Pinecone documents batch starting at %d: %w", start, err)
			}

			if len(payload) > maxUpsertBatchBytes {
				if len(batch) == 0 {
					return totalUpserted, fmt.Errorf(
						"Pinecone document %s exceeds the 2 MiB request limit",
						documents[end]["_id"],
					)
				}
				break
			}

			batch = candidate
			end++
		}

		response, err := c.index.UpsertDocuments(ctx, &pineconesdk.UpsertDocumentsRequest{
			Documents: batch,
		})
		if err != nil {
			return totalUpserted, fmt.Errorf(
				"upsert documents batch starting at %d: %w",
				start,
				err,
			)
		}

		totalUpserted += uint32(response.UpsertedCount)
		start = end
	}

	return totalUpserted, nil
}

func (c *pineconeClient) DeleteVectorsByFilter(ctx context.Context, filter entity.PineconeFilter) error {
	if len(filter) == 0 {
		return fmt.Errorf("Pinecone delete filter cannot be empty")
	}

	_, err := c.index.DeleteDocuments(ctx, &pineconesdk.DeleteDocumentsRequest{
		Filter: map[string]interface{}(filter),
	})
	if err != nil {
		return fmt.Errorf("delete documents from Pinecone: %w", err)
	}

	return nil
}

func (c *pineconeClient) SearchVectors(ctx context.Context, vector []float32) ([]string, error) {
	if len(vector) == 0 {
		return nil, fmt.Errorf("query embedding cannot be empty")
	}

	response, err := c.index.SearchDocuments(ctx, &pineconesdk.SearchDocumentsRequest{
		TopK: c.topK,
		ScoreBy: []pineconesdk.DocumentScoringMethod{
			{
				Type:   pineconesdk.DocumentScoringTypeDenseVector,
				Fields: []string{"embedding"},
				Values: &vector,
			},
		},
		IncludeFields: []string{"content"},
	})
	if err != nil {
		return nil, fmt.Errorf("search documents in Pinecone: %w", err)
	}

	chunks := make([]string, 0, len(response.Matches))
	for _, match := range response.Matches {
		content, ok := match.Fields["content"].(string)
		if ok && content != "" {
			chunks = append(chunks, content)
		}
	}

	return chunks, nil
}
