package gemini

import (
	"context"
	"os"
	"testing"
)

func TestNewClient_MissingAPIKey(t *testing.T) {
	// Save and restore the original API key
	originalKey := os.Getenv("GEMINI_API_KEY")
	defer os.Setenv("GEMINI_API_KEY", originalKey)

	// Test with missing API key
	os.Unsetenv("GEMINI_API_KEY")

	ctx := context.Background()
	_, err := NewClient(ctx)
	if err == nil {
		t.Error("expected error when GEMINI_API_KEY is not set, got nil")
	}

	expectedMsg := "GEMINI_API_KEY environment variable is required"
	if err.Error() != expectedMsg {
		t.Errorf("expected error message '%s', got '%s'", expectedMsg, err.Error())
	}
}

func TestNewClient_WithAPIKey(t *testing.T) {
	// Skip this test if no API key is available
	apiKey := os.Getenv("GEMINI_API_KEY")
	if apiKey == "" {
		t.Skip("GEMINI_API_KEY not set, skipping integration test")
	}

	ctx := context.Background()
	client, err := NewClient(ctx)
	if err != nil {
		t.Fatalf("failed to create Gemini client: %v", err)
	}

	if client == nil {
		t.Fatal("client is nil")
	}
}

func TestNewChatModel_MissingAPIKey(t *testing.T) {
	// Save and restore the original API key
	originalKey := os.Getenv("GEMINI_API_KEY")
	defer os.Setenv("GEMINI_API_KEY", originalKey)

	// Test with missing API key
	os.Unsetenv("GEMINI_API_KEY")

	ctx := context.Background()
	_, err := NewChatModel(ctx)
	if err == nil {
		t.Error("expected error when GEMINI_API_KEY is not set, got nil")
	}
}

func TestNewEmbedder_MissingAPIKey(t *testing.T) {
	// Save and restore the original API key
	originalKey := os.Getenv("GEMINI_API_KEY")
	defer os.Setenv("GEMINI_API_KEY", originalKey)

	// Test with missing API key
	os.Unsetenv("GEMINI_API_KEY")

	ctx := context.Background()
	_, err := NewEmbedder(ctx)
	if err == nil {
		t.Error("expected error when GEMINI_API_KEY is not set, got nil")
	}
}

func TestConstants(t *testing.T) {
	// Verify the constants are set to expected values
	if ChatModelName == "" {
		t.Error("ChatModelName should not be empty")
	}
	if EmbeddingModelName == "" {
		t.Error("EmbeddingModelName should not be empty")
	}

	// These are the current expected values
	expectedChatModel := "gemini-2.5-flash"
	if ChatModelName != expectedChatModel {
		t.Errorf("expected ChatModelName '%s', got '%s'", expectedChatModel, ChatModelName)
	}

	expectedEmbeddingModel := "text-embedding-004"
	if EmbeddingModelName != expectedEmbeddingModel {
		t.Errorf("expected EmbeddingModelName '%s', got '%s'", expectedEmbeddingModel, EmbeddingModelName)
	}
}
