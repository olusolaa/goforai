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

type TavilySearchRequest struct {
	Query string `json:"query" jsonschema:"description=The search query to find information on the internet"`
}

type TavilyResult struct {
	Title   string `json:"title" jsonschema:"description=Title of the search result"`
	URL     string `json:"url" jsonschema:"description=URL of the source"`
	Content string `json:"content" jsonschema:"description=Content excerpt from the source"`
}

type TavilySearchResponse struct {
	Query       string         `json:"query" jsonschema:"description=The search query that was executed"`
	Answer      string         `json:"answer,omitempty" jsonschema:"description=AI-generated summary answer"`
	Results     []TavilyResult `json:"results" jsonschema:"description=Array of search results with structured data"`
	ResultCount int            `json:"result_count" jsonschema:"description=Number of results returned"`
}

func NewTavilySearchTool(ctx context.Context) (tool.BaseTool, error) {
	apiKey := os.Getenv("TAVILY_API_KEY")
	if apiKey == "" {
		return nil, fmt.Errorf("TAVILY_API_KEY environment variable is required")
	}

	return utils.InferTool(
		"search_internet",
		"Search the internet for current information, news, and general knowledge. Returns an AI-generated answer plus structured search results with titles, URLs, and content. Use this for current events, recent news, technical documentation, or any information not in your knowledge base. Always cite sources using the provided URLs.",
		func(ctx context.Context, req *TavilySearchRequest) (*TavilySearchResponse, error) {
			return performTavilySearch(ctx, apiKey, req.Query)
		},
	)
}

func performTavilySearch(ctx context.Context, apiKey, query string) (*TavilySearchResponse, error) {
	client := &http.Client{Timeout: 30 * time.Second}

	reqBody := map[string]interface{}{
		"api_key":        apiKey,
		"query":          query,
		"search_depth":   "basic",
		"include_answer": true,
		"max_results":    5,
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", "https://api.tavily.com/search", bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("HTTP request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API returned status %d: %s", resp.StatusCode, string(body))
	}

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	response := &TavilySearchResponse{
		Query:   query,
		Results: []TavilyResult{},
	}

	if answer, ok := result["answer"].(string); ok && answer != "" {
		response.Answer = answer
	}

	if results, ok := result["results"].([]interface{}); ok {
		for _, res := range results {
			if resMap, ok := res.(map[string]interface{}); ok {
				tavilyResult := TavilyResult{}

				if title, ok := resMap["title"].(string); ok {
					tavilyResult.Title = title
				}
				if url, ok := resMap["url"].(string); ok {
					tavilyResult.URL = url
				}
				if content, ok := resMap["content"].(string); ok {
					tavilyResult.Content = content
				}

				response.Results = append(response.Results, tavilyResult)
			}
		}
		response.ResultCount = len(response.Results)
	}

	return response, nil
}
