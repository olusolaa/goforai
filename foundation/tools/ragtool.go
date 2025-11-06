package tools

import (
	"context"
	"fmt"
	"github.com/olusolaa/goforai/foundation/gemini"
	"strings"

	"github.com/olusolaa/goforai/foundation/chromemdb"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/components/tool/utils"
)

// RAGSearchRequest defines the parameters for searching the knowledge base.
type RAGSearchRequest struct {
	Query string `json:"query" jsonschema:"description=The question to search in the GopherCon Africa 2025 knowledge base"`
}

// RAGSearchResponse contains the retrieved documents from the knowledge base.
type RAGSearchResponse struct {
	Documents string `json:"documents" jsonschema:"description=Relevant documents from the knowledge base"`
	Error     string `json:"error,omitempty" jsonschema:"description=Error message if search failed"`
}

// NewRAGTool creates a new RAG (Retrieval Augmented Generation) tool for searching the knowledge base.
func NewRAGTool(ctx context.Context) (tool.BaseTool, error) {
	embedder, err := gemini.NewEmbedder(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create embedder: %w", err)
	}

	retriever, err := chromemdb.New(ctx, "gophercon-knowledge", embedder,
		chromemdb.WithDBPath("./data/chromem.gob"),
		chromemdb.WithTopK(3))
	if err != nil {
		return nil, fmt.Errorf("failed to create retriever: %w", err)
	}

	return utils.InferTool(
		"search_gophercon_knowledge",
		"Search the GopherCon Africa 2025 knowledge base for information about speakers, talks, schedule, and event details. Use this tool when users ask about GopherCon Africa 2025 specifics. Returns relevant documents with speaker bios, talk descriptions, and event information.",
		func(ctx context.Context, req *RAGSearchRequest) (*RAGSearchResponse, error) {
			docs, err := retriever.Retrieve(ctx, req.Query)
			if err != nil {
				return &RAGSearchResponse{
					Error: fmt.Sprintf("Failed to retrieve documents: %v", err),
				}, nil
			}

			if len(docs) == 0 {
				return &RAGSearchResponse{
					Documents: "No relevant information found in the knowledge base.",
				}, nil
			}

			var result strings.Builder
			result.WriteString(fmt.Sprintf("Found %d relevant documents:\n\n", len(docs)))

			for i, doc := range docs {
				result.WriteString(fmt.Sprintf("=== Document %d ===\n", i+1))
				result.WriteString(doc.Content)
				result.WriteString("\n\n")
			}

			return &RAGSearchResponse{
				Documents: result.String(),
			}, nil
		},
	)
}
