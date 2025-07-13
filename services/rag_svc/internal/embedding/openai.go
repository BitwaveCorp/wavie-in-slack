package embedding

import (
	"context"
	"fmt"
	"time"

	"github.com/sashabaranov/go-openai"
)

// EmbeddingService handles embedding generation
type EmbeddingService struct {
	client *openai.Client
	model  string
}

// NewEmbeddingService creates a new embedding service
func NewEmbeddingService(apiKey, model string) *EmbeddingService {
	client := openai.NewClient(apiKey)
	
	// Use default model if not provided
	if model == "" {
		model = "text-embedding-3-small"
	}
	
	return &EmbeddingService{
		client: client,
		model:  model,
	}
}

// GenerateEmbedding generates an embedding for a text chunk
func (s *EmbeddingService) GenerateEmbedding(ctx context.Context, text string) ([]float32, error) {
	resp, err := s.client.CreateEmbeddings(ctx, openai.EmbeddingRequest{
		Input: []string{text},
		Model: openai.EmbeddingModel(s.model),
	})
	
	if err != nil {
		return nil, fmt.Errorf("failed to generate embedding: %v", err)
	}
	
	if len(resp.Data) == 0 {
		return nil, fmt.Errorf("no embedding data returned")
	}
	
	return resp.Data[0].Embedding, nil
}

// BatchGenerateEmbeddings generates embeddings for multiple text chunks
func (s *EmbeddingService) BatchGenerateEmbeddings(ctx context.Context, texts []string) ([][]float32, error) {
	// OpenAI has limits on batch size, so we process in batches of 20
	batchSize := 20
	var allEmbeddings [][]float32
	
	for i := 0; i < len(texts); i += batchSize {
		end := i + batchSize
		if end > len(texts) {
			end = len(texts)
		}
		
		batch := texts[i:end]
		
		resp, err := s.client.CreateEmbeddings(ctx, openai.EmbeddingRequest{
			Input: batch,
			Model: openai.EmbeddingModel(s.model),
		})
		
		if err != nil {
			return nil, fmt.Errorf("failed to generate batch embeddings: %v", err)
		}
		
		for _, data := range resp.Data {
			allEmbeddings = append(allEmbeddings, data.Embedding)
		}
		
		// Add a small delay to avoid rate limits
		if end < len(texts) {
			time.Sleep(200 * time.Millisecond)
		}
	}
	
	return allEmbeddings, nil
}

// EstimateEmbeddingCost estimates the cost of generating embeddings
// Based on OpenAI's pricing for text-embedding-3-small: $0.00002 per 1K tokens
func (s *EmbeddingService) EstimateEmbeddingCost(tokenCount int) float64 {
	// Convert to thousands of tokens
	thousands := float64(tokenCount) / 1000.0
	
	// Cost per 1K tokens
	costPer1K := 0.00002
	
	return thousands * costPer1K
}
