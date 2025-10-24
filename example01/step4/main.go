// Step 4: Adding Actions (Tools) to Create a Complete Agent
//
// This is the final evolution. We take our interactive, streaming, RAG-enabled
// agent from Step 3 and give it a more powerful brain: Eino's `react.Agent`.
// Now the agent can intelligently choose: should I answer from my knowledge
// base (RAG), or use a tool like internet search?
//
// The narrative is about the power of composition. We've built all the pieces,
// and now we snap them together into a final, sophisticated application that
// showcases the full power of Go for building production-grade AI.

package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/cloudwego/eino-ext/components/embedding/gemini"
	geminiModel "github.com/cloudwego/eino-ext/components/model/gemini"
	"google.golang.org/genai"

	"github.com/cloudwego/eino/components/embedding"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/components/prompt"
	"github.com/cloudwego/eino/components/retriever"
	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/components/tool/utils"
	"github.com/cloudwego/eino/flow/agent/react"
	"github.com/cloudwego/eino/schema"
	"github.com/olusolaa/goforai/foundation/chromemdb"
)

// ---
// Step 1: The Orchestrator (The Final Application)
// ---

func main() {
	if err := run(context.Background()); err != nil {
		log.Fatal(err)
	}
}

func run(ctx context.Context) error {
	// *** SAME: Create all our modular dependencies. ***
	clients, err := newAIClients(ctx)
	if err != nil {
		return err
	}
	ragRetriever, err := newRetriever(ctx, clients.embedder)
	if err != nil {
		return err
	}
	ragTemplate, err := newRAGTemplate()
	if err != nil {
		return err
	}

	// ************** NEW: Build the tool registry for our new tools. **************
	toolRegistry, err := newToolRegistry(ctx)
	if err != nil {
		return err
	}

	// ******** CHANGED: Build the final, most powerful agent with all components. ********
	agent := NewAgent(clients.chatModel, ragRetriever, ragTemplate, toolRegistry, os.Stdin, os.Stdout)
	return agent.Run(ctx)
}

// ---
// Step 2: The Core Logic - An Agent with RAG and Tools
// ---

// ******* CHANGED: System prompt now reflects the agent's full capabilities. *******
const systemPrompt = `You are an assistant with access to a knowledge base and internet search. Use the knowledge base for GopherCon Africa questions. Use internet search for all other topics.`

// ******* CHANGED: The agent now holds the powerful `react.Agent` as its brain. *******

type Agent struct {
	reactAgent *react.Agent        // The decision-making brain.
	retriever  retriever.Retriever // The knowledge base.
	template   prompt.ChatTemplate // Formats prompts with knowledge.
	scanner    *bufio.Scanner
	out        io.Writer
}

// ************ We build the `react.Agent` here, giving it the tools. **************

func NewAgent(m model.ToolCallingChatModel, r retriever.Retriever, t prompt.ChatTemplate, toolRegistry map[string]tool.BaseTool, in io.Reader, out io.Writer) *Agent {
	toolsList := make([]tool.BaseTool, 0, len(toolRegistry))
	for _, tool := range toolRegistry {
		toolsList = append(toolsList, tool)
	}
	config := &react.AgentConfig{MaxStep: 10, ToolCallingModel: m}
	config.ToolsConfig.Tools = toolsList
	reactAgent, _ := react.NewAgent(context.Background(), config)

	return &Agent{
		reactAgent: reactAgent,
		retriever:  r,
		template:   t,
		scanner:    bufio.NewScanner(in),
		out:        out,
	}
}

func (a *Agent) Run(ctx context.Context) error {
	conversation := []*schema.Message{schema.SystemMessage(systemPrompt)}
	fmt.Fprintf(a.out, "\nChat with a RAG + Tool-powered agent (use 'ctrl-c' to quit)\n")

	for {
		fmt.Fprintf(a.out, "%s\nYou%s: ", colorBlue, colorReset)
		if !a.scanner.Scan() {
			break
		}
		userInput := a.scanner.Text()
		if userInput == "" {
			continue
		}

		// *** SAME: We still perform the RAG step first to gather context. ***
		docs, err := a.retriever.Retrieve(ctx, userInput)
		if err != nil {
			fmt.Fprintf(a.out, "\n\n%sERROR: %s%s\n\n", colorRed, err, colorReset)
			continue
		}

		// Build context string from retrieved documents
		var sb strings.Builder
		for i, doc := range docs {
			if i > 0 {
				sb.WriteString("\n---\n")
			}
			sb.WriteString(doc.Content)
		}

		messages, err := a.template.Format(ctx, map[string]any{"context": sb.String(), "question": userInput})
		if err != nil {
			fmt.Fprintf(a.out, "\n\n%sERROR: %s%s\n\n", colorRed, err, colorReset)
			continue
		}

		// Extract the user message with context from the template (skip system message)
		userMessageWithContext := messages[len(messages)-1]
		conversation = append(conversation, userMessageWithContext)
		fmt.Fprintf(a.out, "%s\n%s%s: ", colorYellow, chatModelName, colorReset)

		// ******** CHANGED: We hand off to the `react.Agent` for the final decision. ********
		streamReader, err := a.reactAgent.Stream(ctx, conversation)
		if err != nil {
			fmt.Fprintf(a.out, "\n\n%sERROR: %s%s\n\n", colorRed, err, colorReset)
			continue
		}

		// *** The rest of the code is the same polished UX from Step 2 & 3! ***
		done := make(chan struct{})
		go a.showSpinner(done)

		var chunks []*schema.Message
		firstChunk := true
		for {
			chunk, err := streamReader.Recv()
			if err != nil {
				if firstChunk {
					close(done)
				}
				if err == io.EOF {
					break
				}
				fmt.Fprintf(a.out, "\n\n%sERROR: %s%s\n\n", colorRed, err, colorReset)
				break
			}
			if firstChunk {
				close(done)
				time.Sleep(10 * time.Millisecond)
				firstChunk = false
			}
			fmt.Fprint(a.out, chunk.Content)
			chunks = append(chunks, chunk)
		}
		fmt.Fprint(a.out, "\n")

		if len(chunks) > 0 {
			fullMsg, _ := schema.ConcatMessages(chunks)
			conversation = append(conversation, fullMsg)
		}
	}
	return a.scanner.Err()
}

// *** Helper method for the concurrent spinner (Unchanged from Step 2). ***
func (a *Agent) showSpinner(done <-chan struct{}) {
	spinner := `|/-\`
	i := 0
	for {
		select {
		case <-done:
			fmt.Fprint(a.out, "\b")
			return
		default:
			fmt.Fprintf(a.out, "%c", spinner[i])
			i = (i + 1) % len(spinner)
			time.Sleep(100 * time.Millisecond)
			fmt.Fprint(a.out, "\b")
		}
	}
}

// ---
// Step 3: Dependencies and Factories
// ---
const (
	chatModelName      = "gemini-2.5-flash"
	embeddingModelName = "text-embedding-004"
	dbPath             = "./data/chromem.gob"
	colorBlue          = "\u001b[94m"
	colorYellow        = "\u001b[93m"
	colorRed           = "\u001b[91m"
	colorReset         = "\u001b[0m"
)

type aiClients struct {
	chatModel model.ToolCallingChatModel
	embedder  embedding.Embedder
}

func newAIClients(ctx context.Context) (*aiClients, error) {
	apiKey := os.Getenv("GEMINI_API_KEY")
	if apiKey == "" {
		return nil, errors.New("GEMINI_API_KEY environment variable not set")
	}
	client, err := genai.NewClient(ctx, &genai.ClientConfig{APIKey: apiKey})
	if err != nil {
		return nil, fmt.Errorf("failed to create gemini client: %w", err)
	}
	chatModel, err := geminiModel.NewChatModel(ctx, &geminiModel.Config{Client: client, Model: chatModelName})
	if err != nil {
		return nil, fmt.Errorf("failed to create chat model: %w", err)
	}
	embedder, err := gemini.NewEmbedder(ctx, &gemini.EmbeddingConfig{Client: client, Model: embeddingModelName})
	if err != nil {
		return nil, fmt.Errorf("failed to create embedder: %w", err)
	}
	return &aiClients{chatModel: chatModel, embedder: embedder}, nil
}

func newRetriever(ctx context.Context, embedder embedding.Embedder) (retriever.Retriever, error) {
	return chromemdb.New(ctx, "gophercon-knowledge", embedder,
		chromemdb.WithDBPath(dbPath),
		chromemdb.WithTopK(3),
	)
}

func newRAGTemplate() (prompt.ChatTemplate, error) {
	return prompt.FromMessages(
		schema.FString,
		schema.SystemMessage(systemPrompt),
		schema.UserMessage(`Based on the following context if it is relevant, answer the question.

Context:
{context}

Question: {question}`),
	), nil
}

// ******** NEW: A factory to build our agent's complete "toolbox". ************
func newToolRegistry(ctx context.Context) (map[string]tool.BaseTool, error) {
	searchTool, err := NewTavilySearchTool(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create search tool: %w", err)
	}
	return map[string]tool.BaseTool{"search_internet": searchTool}, nil
}

// ******** NEW: The Type-Safe Tool Implementation (Unchanged and Reusable) ********

type TavilySearchRequest struct {
	Query string `json:"query" jsonschema:"description=The search query to find information on the internet."`
}
type TavilySearchResponse struct {
	Answer  string `json:"answer,omitempty" jsonschema:"description=AI-generated summary of search results."`
	Results []struct {
		URL     string `json:"url" jsonschema:"description=URL of the source."`
		Content string `json:"content" jsonschema:"description=Content excerpt from the source."`
	} `json:"results" jsonschema:"description=Array of search results."`
}

func NewTavilySearchTool(ctx context.Context) (tool.BaseTool, error) {
	apiKey := os.Getenv("TAVILY_API_KEY")
	if apiKey == "" {
		return nil, errors.New("TAVILY_API_KEY environment variable is required")
	}
	return utils.InferTool(
		"search_internet",
		"Searches the internet for current events and information.",
		func(ctx context.Context, req *TavilySearchRequest) (*TavilySearchResponse, error) {
			return performTavilySearch(ctx, apiKey, req.Query)
		},
	)
}

func performTavilySearch(ctx context.Context, apiKey, query string) (*TavilySearchResponse, error) {
	client := &http.Client{Timeout: 15 * time.Second}
	reqBody := map[string]interface{}{"api_key": apiKey, "query": query, "include_answer": true, "max_results": 3}
	jsonData, _ := json.Marshal(reqBody)
	req, _ := http.NewRequestWithContext(ctx, "POST", "https://api.tavily.com/search", bytes.NewBuffer(jsonData))
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("HTTP request failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API error: %s", string(body))
	}
	var result TavilySearchResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}
	return &result, nil
}
