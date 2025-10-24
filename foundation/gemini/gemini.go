package gemini

import (
	"context"
	"fmt"
	"os"

	"github.com/cloudwego/eino-ext/components/embedding/gemini"
	geminiModel "github.com/cloudwego/eino-ext/components/model/gemini"
	"github.com/cloudwego/eino/components/embedding"
	"github.com/cloudwego/eino/components/model"
	"google.golang.org/genai"
)

const (
	ChatModelName      = "gemini-2.5-flash"
	EmbeddingModelName = "text-embedding-004"
)

// NewClient creates a new Gemini API client.
func NewClient(ctx context.Context) (*genai.Client, error) {
	apiKey := os.Getenv("GEMINI_API_KEY")
	if apiKey == "" {
		return nil, fmt.Errorf("GEMINI_API_KEY environment variable is required")
	}

	client, err := genai.NewClient(ctx, &genai.ClientConfig{
		APIKey:  apiKey,
		Backend: genai.BackendGeminiAPI,
	})
	if err != nil {

		return nil, fmt.Errorf("failed to create Gemini client: %w", err)
	}

	return client, nil
}

// NewChatModel creates a new Gemini chat model.
func NewChatModel(ctx context.Context) (model.ToolCallingChatModel, error) {
	client, err := NewClient(ctx)
	if err != nil {
		return nil, err
	}

	config := &geminiModel.Config{
		Client: client,
		Model:  ChatModelName,
	}

	chatModel, err := geminiModel.NewChatModel(ctx, config)
	if err != nil {
		return nil, fmt.Errorf("failed to create chat model: %w", err)
	}

	return chatModel, nil
}

// NewEmbedder creates a new Gemini embedder for vector operations.
func NewEmbedder(ctx context.Context) (embedding.Embedder, error) {
	client, err := NewClient(ctx)
	if err != nil {
		return nil, err
	}

	config := &gemini.EmbeddingConfig{
		Client: client,
		Model:  EmbeddingModelName,
	}

	embedder, err := gemini.NewEmbedder(ctx, config)
	if err != nil {
		return nil, fmt.Errorf("failed to create embedder: %w", err)
	}

	return embedder, nil
}
