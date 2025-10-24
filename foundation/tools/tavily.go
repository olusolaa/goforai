package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/components/tool/utils"
)

// --- User-Facing Request/Response Structs ---

type TavilySearchRequest struct {
	Query       string `json:"query" jsonschema:"description=The search query to find information on the internet."`
	SearchDepth string `json:"search_depth,omitempty" jsonschema:"description=The depth of the search. Can be 'basic' or 'advanced'. Defaults to 'basic'."`
	MaxResults  *int   `json:"max_results,omitempty" jsonschema:"description=The maximum number of results to return. Defaults to 5."`
}

type TavilySearchResponse struct {
	Query       string         `json:"query" jsonschema:"description=The search query that was executed."`
	Answer      string         `json:"answer,omitempty" jsonschema:"description=AI-generated summary answer, if available."`
	Results     []TavilyResult `json:"results" jsonschema:"description=Array of search results with structured data."`
	ResultCount int            `json:"result_count" jsonschema:"description=Number of results returned."`
	Error       string         `json:"error,omitempty" jsonschema:"description=Error message if the search failed."`
}

type TavilyResult struct {
	Title   string `json:"title"`
	URL     string `json:"url"`
	Content string `json:"content"`
}

// --- Internal Structs for Safe API Interaction ---

// tavilyAPIBody mirrors the structure of the JSON request sent to the Tavily API.
type tavilyAPIBody struct {
	APIKey        string `json:"api_key"`
	Query         string `json:"query"`
	SearchDepth   string `json:"search_depth,omitempty"`
	IncludeAnswer bool   `json:"include_answer"`
	MaxResults    int    `json:"max_results,omitempty"`
}

// tavilyAPIResponse mirrors the successful JSON response from the Tavily API.
type tavilyAPIResponse struct {
	Answer  string         `json:"answer"`
	Query   string         `json:"query"`
	Results []TavilyResult `json:"results"`
}

// tavilyErrorResponse mirrors the error JSON response from the Tavily API.
type tavilyErrorResponse struct {
	Error string `json:"error"`
}

// TavilyTool holds the persistent state for the tool, like the API key and HTTP client.
type TavilyTool struct {
	apiKey     string
	httpClient *http.Client
}

func NewTavilySearchTool(ctx context.Context) (tool.BaseTool, error) {
	apiKey := os.Getenv("TAVILY_API_KEY")
	if apiKey == "" {
		return nil, fmt.Errorf("TAVILY_API_KEY environment variable is required")
	}

	// Create a single, reusable HTTP client.
	client := &http.Client{
		Timeout: 30 * time.Second,
	}

	impl := &TavilyTool{
		apiKey:     apiKey,
		httpClient: client,
	}

	return utils.InferTool(
		"search_internet",
		"Search the internet for current information, news, and general knowledge. Returns an AI-generated answer "+
			"plus structured search results. Allows for 'basic' or 'advanced' search depth. Always cite sources using the provided URLs."+
			"Use github.com to search for GitHub repositories and LinkedIn to search for peoples profiles.",
		impl.PerformSearch,
	)
}

func (t *TavilyTool) PerformSearch(ctx context.Context, req *TavilySearchRequest) (*TavilySearchResponse, error) {
	searchDepth := "basic"
	if req.SearchDepth == "advanced" {
		searchDepth = "advanced"
	}
	maxResults := 5
	if req.MaxResults != nil {
		maxResults = *req.MaxResults
	}

	apiReqBody := tavilyAPIBody{
		APIKey:        t.apiKey,
		Query:         req.Query,
		SearchDepth:   searchDepth,
		IncludeAnswer: true,
		MaxResults:    maxResults,
	}

	jsonData, err := json.Marshal(apiReqBody)
	if err != nil {
		return &TavilySearchResponse{Error: fmt.Sprintf("failed to marshal request: %v", err)}, nil
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://api.tavily.com/search", bytes.NewBuffer(jsonData))
	if err != nil {
		return &TavilySearchResponse{Error: fmt.Sprintf("failed to create request: %v", err)}, nil
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := t.httpClient.Do(httpReq)
	if err != nil {
		return &TavilySearchResponse{Error: fmt.Sprintf("HTTP request failed: %v", err)}, nil
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		var errResp tavilyErrorResponse
		if json.NewDecoder(resp.Body).Decode(&errResp) == nil && errResp.Error != "" {
			return &TavilySearchResponse{Error: fmt.Sprintf("API error: %s (status %d)", errResp.Error, resp.StatusCode)}, nil
		}
		// Fallback for unexpected error formats
		body, _ := io.ReadAll(io.MultiReader(bytes.NewReader(jsonData), resp.Body)) // Reset reader after decode attempt
		return &TavilySearchResponse{Error: fmt.Sprintf("API returned non-200 status: %d, body: %s", resp.StatusCode, string(body))}, nil
	}

	var apiResp tavilyAPIResponse
	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		return &TavilySearchResponse{Error: fmt.Sprintf("failed to decode successful response: %v", err)}, nil
	}

	return &TavilySearchResponse{
		Query:       apiResp.Query,
		Answer:      apiResp.Answer,
		Results:     apiResp.Results,
		ResultCount: len(apiResp.Results),
	}, nil
}
