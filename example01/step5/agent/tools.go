package agent

import (
	"context"
	"fmt"
	"log"

	"github.com/cloudwego/eino/components/tool"
	"github.com/olusolaa/goforai/foundation/tools"
)

// setupTools initializes and returns the list of tools for the agent.
// It includes logic for falling back to alternative tools if primaries fail.
func setupTools(ctx context.Context) ([]tool.BaseTool, error) {
	ragTool, err := tools.NewRAGTool(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create RAG tool: %w", err)
	}
	readFileTool, err := tools.NewReadFileTool(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create read file tool: %w", err)
	}
	searchFilesTool, err := tools.NewSearchFilesTool(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create search files tool: %w", err)
	}
	editFileTool, err := tools.NewEditFileTool(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create edit file tool: %w", err)
	}
	gitCloneTool, err := tools.NewGitCloneTool(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create git clone tool: %w", err)
	}

	searchTool := setupSearchTool(ctx)

	toolsList := []tool.BaseTool{
		searchFilesTool,
		readFileTool,
		editFileTool,
		gitCloneTool,
		ragTool,
	}
	if searchTool != nil {
		toolsList = append(toolsList, searchTool)
	}

	return toolsList, nil
}

// setupSearchTool attempts to create the primary search tool (Tavily)
// and falls back to a secondary one (DuckDuckGo) if it fails.
func setupSearchTool(ctx context.Context) tool.BaseTool {
	tavilyTool, err := tools.NewTavilySearchTool(ctx)
	if err == nil {
		log.Println("✅ Using Tavily for web search")
		return tavilyTool
	}
	log.Printf("ℹ️ Tavily search not available (%v), falling back to DuckDuckGo", err)

	ddgTool, err := tools.NewDuckDuckGoSearchTool(ctx)
	if err == nil {
		log.Println("✅ Using DuckDuckGo for web search")
		return ddgTool
	}
	log.Printf("⚠️ Could not initialize any web search tool (%v)", err)
	return nil
}
