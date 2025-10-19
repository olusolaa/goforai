package tools

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/components/tool/utils"
)

type DuckDuckGoSearchRequest struct {
	Query string `json:"query" jsonschema:"description=The search query to find information on the internet"`
}

type DuckDuckGoSearchResponse struct {
	Results string `json:"results" jsonschema:"description=Search results from the internet with sources"`
}

func NewDuckDuckGoSearchTool(ctx context.Context) (tool.BaseTool, error) {
	return utils.InferTool(
		"search_internet",
		"Search the internet for current information, news, GitHub repositories, and general knowledge. Use this for current events, recent news, or information not in the GopherCon knowledge base. Returns top search results with URLs.",
		func(ctx context.Context, req *DuckDuckGoSearchRequest) (*DuckDuckGoSearchResponse, error) {
			return performDuckDuckGoSearch(ctx, req.Query)
		},
	)
}

func performDuckDuckGoSearch(ctx context.Context, query string) (*DuckDuckGoSearchResponse, error) {
	// DuckDuckGo HTML search (no API key needed!)
	searchURL := fmt.Sprintf("https://html.duckduckgo.com/html/?q=%s", url.QueryEscape(query))

	client := &http.Client{}
	req, err := http.NewRequestWithContext(ctx, "GET", searchURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Set a user agent to avoid being blocked
	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; GopherConBot/1.0)")

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("search request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("search returned status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	// Parse HTML results (simple extraction)
	results := parseSearchResults(string(body))

	if len(results) == 0 {
		return &DuckDuckGoSearchResponse{
			Results: fmt.Sprintf("No results found for: %s", query),
		}, nil
	}

	// Format results
	var formattedResults strings.Builder
	formattedResults.WriteString(fmt.Sprintf("Search results for '%s':\n\n", query))

	for i, result := range results {
		if i >= 5 { // Limit to top 5 results
			break
		}
		formattedResults.WriteString(fmt.Sprintf("%d. %s\n", i+1, result.Title))
		formattedResults.WriteString(fmt.Sprintf("   URL: %s\n", result.URL))
		if result.Snippet != "" {
			formattedResults.WriteString(fmt.Sprintf("   %s\n", result.Snippet))
		}
		formattedResults.WriteString("\n")
	}

	return &DuckDuckGoSearchResponse{Results: formattedResults.String()}, nil
}

type SearchResult struct {
	Title   string
	URL     string
	Snippet string
}

func parseSearchResults(html string) []SearchResult {
	var results []SearchResult

	// Extract result divs (DuckDuckGo HTML structure)
	resultPattern := regexp.MustCompile(`<div class="result[^"]*">.*?</div>`)
	resultDivs := resultPattern.FindAllString(html, -1)

	for _, div := range resultDivs {
		result := SearchResult{}

		// Extract title
		titlePattern := regexp.MustCompile(`<a class="result__a" href="[^"]*">([^<]+)</a>`)
		if matches := titlePattern.FindStringSubmatch(div); len(matches) > 1 {
			result.Title = cleanText(matches[1])
		}

		// Extract URL
		urlPattern := regexp.MustCompile(`<a class="result__url" href="([^"]+)"`)
		if matches := urlPattern.FindStringSubmatch(div); len(matches) > 1 {
			result.URL = cleanText(matches[1])
		}

		// Extract snippet
		snippetPattern := regexp.MustCompile(`<a class="result__snippet"[^>]*>([^<]+)</a>`)
		if matches := snippetPattern.FindStringSubmatch(div); len(matches) > 1 {
			result.Snippet = cleanText(matches[1])
		}

		// Only add if we have at least a title and URL
		if result.Title != "" && result.URL != "" {
			results = append(results, result)
		}
	}

	// Fallback: try alternative parsing if no results
	if len(results) == 0 {
		results = parseAlternativeFormat(html)
	}

	return results
}

func parseAlternativeFormat(html string) []SearchResult {
	var results []SearchResult

	// Alternative pattern for links
	linkPattern := regexp.MustCompile(`<a[^>]+href="([^"]+)"[^>]*>([^<]+)</a>`)
	matches := linkPattern.FindAllStringSubmatch(html, -1)

	seen := make(map[string]bool)
	for _, match := range matches {
		if len(match) < 3 {
			continue
		}

		url := match[1]
		title := cleanText(match[2])

		// Skip internal DuckDuckGo links
		if strings.Contains(url, "duckduckgo.com") {
			continue
		}

		// Skip duplicates
		if seen[url] {
			continue
		}

		// Must be a real URL
		if !strings.HasPrefix(url, "http") {
			continue
		}

		seen[url] = true
		results = append(results, SearchResult{
			Title: title,
			URL:   url,
		})

		if len(results) >= 10 {
			break
		}
	}

	return results
}

func cleanText(s string) string {
	// Remove HTML entities
	s = strings.ReplaceAll(s, "&amp;", "&")
	s = strings.ReplaceAll(s, "&lt;", "<")
	s = strings.ReplaceAll(s, "&gt;", ">")
	s = strings.ReplaceAll(s, "&quot;", "\"")
	s = strings.ReplaceAll(s, "&#39;", "'")
	s = strings.ReplaceAll(s, "&nbsp;", " ")

	// Trim whitespace
	s = strings.TrimSpace(s)

	return s
}
