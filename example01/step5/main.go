// Step 5: Full Integration with Eino Graph Orchestration
//
// This is where Eino proves its value! We're combining:
//  - Terminal chat loop (from steps 1 & 2)
//  - RAG retrieval (from step 3)
//  - Tool calling (from step 4)
//  - All orchestrated by Eino's graph + ReAct pattern
//
// WHY EINO NOW? Steps 1-4 were simple enough for manual orchestration.
// But orchestrating RAG + Tools + Chat + Memory manually would require
// complex conditional logic, loops, and state management.
// Eino's graph makes it declarative and manageable.
//
// # Running the example:
//
//	go run examples/step5/main.go
//
// # Requirements:
//
//	GEMINI_API_KEY environment variable must be set
//	data/chromem.gob must exist (run: go run ./cmd/indexing)

package main

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"strings"
	"time"

	"github.com/olusolaa/goforai/foundation"
	"github.com/olusolaa/goforai/foundation/tools"

	"github.com/cloudwego/eino/callbacks"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/components/prompt"
	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/flow/agent/react"
	"github.com/cloudwego/eino/schema"
)

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	scanner := bufio.NewScanner(os.Stdin)
	getUserMessage := func() (string, bool) {
		if !scanner.Scan() {
			return "", false
		}
		return scanner.Text(), true
	}

	agent, err := NewAgent(getUserMessage)
	if err != nil {
		return fmt.Errorf("failed to create agent: %w", err)
	}

	return agent.Run(context.Background())
}

// =============================================================================
// Agent Structure

type Agent struct {
	graph          compose.Runnable[*UserMessage, *schema.Message]
	getUserMessage func() (string, bool)
	conversation   []*schema.Message
}

// UserMessage is the input structure for our agent graph
type UserMessage struct {
	Query   string
	History []*schema.Message
}

// =============================================================================
// Agent Construction - THE EINO MAGIC HAPPENS HERE

func NewAgent(getUserMessage func() (string, bool)) (*Agent, error) {
	ctx := context.Background()

	// BUILD THE EINO GRAPH - this is what makes complex orchestration simple!
	graph, err := buildAgentGraph(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to build agent graph: %w", err)
	}

	return &Agent{
		graph:          graph,
		getUserMessage: getUserMessage,
		conversation:   make([]*schema.Message, 0),
	}, nil
}

// =============================================================================
// Graph Construction - Where Eino Orchestration Happens

func buildAgentGraph(ctx context.Context) (compose.Runnable[*UserMessage, *schema.Message], error) {
	// Simplified graph - RAG is now a TOOL, not automatic!
	const (
		InputToHistory = "InputToHistory"
		ChatTemplate   = "ChatTemplate"
		ReactAgent     = "ReactAgent"
	)

	// CREATE THE GRAPH - Simpler without automatic retrieval
	g := compose.NewGraph[*UserMessage, *schema.Message]()

	// NODE 1: Extract history and prepare variables
	_ = g.AddLambdaNode(InputToHistory, compose.InvokableLambda(extractVariables))

	// NODE 2: CHAT TEMPLATE - Just history + query
	chatTemplate, err := createChatTemplate(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create chat template: %w", err)
	}

	_ = g.AddChatTemplateNode(ChatTemplate, chatTemplate)

	// NODE 3: REACT AGENT - With RAG as a tool!
	reactAgent, err := createReactAgent(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create react agent: %w", err)
	}

	_ = g.AddLambdaNode(ReactAgent, reactAgent)

	// CONNECT THE NODES - Simplified linear flow
	_ = g.AddEdge(compose.START, InputToHistory)
	_ = g.AddEdge(InputToHistory, ChatTemplate) // History → Template
	_ = g.AddEdge(ChatTemplate, ReactAgent)     // Template → Agent
	_ = g.AddEdge(ReactAgent, compose.END)      // Agent → Output

	// COMPILE THE GRAPH
	runnable, err := g.Compile(ctx, compose.WithGraphName("GopherConAgent"))
	if err != nil {
		return nil, fmt.Errorf("failed to compile graph: %w", err)
	}

	return runnable, nil
}

// =============================================================================
// Graph Component Constructors

// Lambda function to extract variables for chat template
func extractVariables(ctx context.Context, input *UserMessage) (map[string]any, error) {
	return map[string]any{
		"content": input.Query,
		"history": input.History,
		"date":    time.Now().Format("2006-01-02 15:04:05"),
	}, nil
}

// Creates the chat template
func createChatTemplate(ctx context.Context) (prompt.ChatTemplate, error) {
	systemPrompt := `You are an expert Go coding assistant with access to powerful development tools.

Core Principles:
- Be proactive: use tools to find information instead of asking the user
- Be decisive: if you have the right tool, use it immediately
- Follow tool responses: they contain paths, metadata, and instructions - use them
- Learn from errors: read error messages carefully and adapt your approach
- Never apologize repeatedly: fix the issue and move forward

When using tools:
- Tools return structured data (paths, line numbers, file sizes) - pay attention to these
- Error messages contain specific instructions - follow them exactly
- If a tool fails twice with the same error, stop and explain what you found

Current Date: {date}
`

	template := prompt.FromMessages(
		schema.FString,
		schema.SystemMessage(systemPrompt),
		schema.MessagesPlaceholder("history", true),
		schema.UserMessage("{content}"),
	)

	return template, nil
}

// Creates the ReAct agent with tools
func createReactAgent(ctx context.Context) (*compose.Lambda, error) {
	// Get the chat model
	chatModel, err := foundation.NewChatModel(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create chat model: %w", err)
	}

	// Initialize all tools for a CODING AGENT (including RAG as a tool!)
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

	// Try Tavily first (best quality), fallback to DuckDuckGo (no API key needed)
	var searchTool tool.BaseTool
	tavilyTool, err := tools.NewTavilySearchTool(ctx)
	if err != nil {
		// Tavily not available, use DuckDuckGo as fallback
		fmt.Println("ℹ️  Using DuckDuckGo search (Tavily not available)")
		ddgTool, err := tools.NewDuckDuckGoSearchTool(ctx)
		if err != nil {
			// No search available at all - continue without it
			fmt.Println("⚠️  No search tool available")
			toolsList := []tool.BaseTool{readFileTool, searchFilesTool, editFileTool, gitCloneTool}
			return buildReactAgent(ctx, chatModel, toolsList)
		}
		searchTool = ddgTool
	} else {
		fmt.Println("✓ Using Tavily search")
		searchTool = tavilyTool
	}

	// Order tools by frequency of use (most common first helps model selection)
	toolsList := []tool.BaseTool{
		searchFilesTool, // Most common: finding files
		readFileTool,    // Second most: reading code
		editFileTool,    // Third: making changes
		searchTool,      // External info when needed
		gitCloneTool,    // Occasional: cloning repos
		ragTool,         // Fallback: knowledge base
	}

	return buildReactAgent(ctx, chatModel, toolsList)
}

// Helper to build the ReAct agent with a list of tools
func buildReactAgent(ctx context.Context, chatModel model.ToolCallingChatModel, toolsList []tool.BaseTool) (*compose.Lambda, error) {
	// Configure the ReAct agent with optimized settings
	config := &react.AgentConfig{
		MaxStep:            20, // Allow enough steps for complex multi-tool tasks
		ToolCallingModel:   chatModel,
		ToolReturnDirectly: map[string]struct{}{},
	}
	config.ToolsConfig.Tools = toolsList

	// Create the ReAct agent instance
	reactAgent, err := react.NewAgent(ctx, config)
	if err != nil {
		return nil, fmt.Errorf("failed to create react agent: %w", err)
	}

	// Wrap in a lambda for graph compatibility
	lambda, err := compose.AnyLambda(reactAgent.Generate, reactAgent.Stream, nil, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create lambda: %w", err)
	}

	return lambda, nil
}

// =============================================================================
// Callback Handler for Showing Tool Calls with Better UX

func createToolCallbackHandler() callbacks.Handler {
	builder := callbacks.NewHandlerBuilder()

	// Show tool execution start
	builder.OnStartFn(func(ctx context.Context, info *callbacks.RunInfo, input callbacks.CallbackInput) context.Context {
		switch info.Component {
		case "Tool":
			// Show tool calls with icons for better UX
			var icon string
			switch info.Name {
			case "search_files":
				icon = "🔍"
			case "read_file":
				icon = "📖"
			case "edit_go_file":
				icon = "✏️"
			case "search_internet":
				icon = "🌐"
			case "gitclone":
				icon = "📥"
			default:
				icon = "🛠️"
			}

			if info.Name == "gitclone" {
				fmt.Printf("\n\n\u001b[92m[%s  %s] (cloning repository...)\u001b[0m", icon, info.Name)
			} else {
				fmt.Printf("\n\n\u001b[92m[%s  %s]\u001b[0m", icon, info.Name)
			}
		}
		return ctx
	})

	// Show tool completion
	builder.OnEndFn(func(ctx context.Context, info *callbacks.RunInfo, output callbacks.CallbackOutput) context.Context {
		switch info.Component {
		case "Tool":
			fmt.Printf(" \u001b[92m✓\u001b[0m")
		}
		return ctx
	})

	// Show errors clearly
	builder.OnErrorFn(func(ctx context.Context, info *callbacks.RunInfo, err error) context.Context {
		if info.Component == "Tool" {
			fmt.Printf(" \u001b[91m✗ Error: %v\u001b[0m\n", err)
		}
		return ctx
	})

	return builder.Build()
}

// =============================================================================
// Agent Execution

func (a *Agent) Run(ctx context.Context) error {
	fmt.Printf("\n\u001b[96m╔══════════════════════════════════════════════════════════════╗\u001b[0m")
	fmt.Printf("\n\u001b[96m║       🤖 Expert Go Coding Agent - Powered by Eino          ║\u001b[0m")
	fmt.Printf("\n\u001b[96m╚══════════════════════════════════════════════════════════════╝\u001b[0m\n")
	fmt.Printf("\nModel: %s | Max Steps: 15 | Tools: 6\n", foundation.ChatModelName)
	fmt.Printf("\n\u001b[92m🛠️  Available Capabilities:\u001b[0m\n")
	fmt.Printf("  🔍 Smart file search (with line numbers & snippets)\n")
	fmt.Printf("  📖 Read files (with metadata & range support)\n")
	fmt.Printf("  ✏️  Edit code (context-aware search & replace)\n")
	fmt.Printf("  🌐 Search internet (Tavily with structured results)\n")
	fmt.Printf("  📥 Clone repos (with smart path guidance)\n")
	fmt.Printf("  📚 GopherCon Africa 2025 knowledge base\n")
	fmt.Printf("\n\u001b[93m💡 Example Queries:\u001b[0m\n")
	fmt.Printf("  • 'Clone github.com/cloudwego/eino and find RAG examples'\n")
	fmt.Printf("  • 'Clone Alao Olusola repo and show me the structure'\n")
	fmt.Printf("  • 'Search for all files containing \"oauth2.Config\"'\n")
	fmt.Printf("  • 'Read main.go lines 100-150'\n")
	fmt.Printf("  • 'Who are the GopherCon Africa 2025 speakers?'\n")
	fmt.Printf("\nType 'exit' to quit\n")
	fmt.Println(strings.Repeat("─", 65))

	for {
		// Get user input
		fmt.Print("\n\u001b[94m🧑 You\u001b[0m: ")
		userInput, ok := a.getUserMessage()
		if !ok {
			break
		}

		userInput = strings.TrimSpace(userInput)
		if userInput == "" {
			continue
		}

		if strings.ToLower(userInput) == "exit" || strings.ToLower(userInput) == "quit" {
			fmt.Println("\n👋 Goodbye!")
			return nil
		}

		// Create the input for the graph
		input := &UserMessage{
			Query:   userInput,
			History: a.conversation,
		}

		// RUN THE GRAPH with callback to show tool calls
		fmt.Printf("\n\u001b[93m🤖 %s\u001b[0m: ", foundation.ChatModelName)

		cbHandler := createToolCallbackHandler()

		streamReader, err := a.graph.Stream(ctx, input, compose.WithCallbacks(cbHandler))
		if err != nil {
			fmt.Printf("\n\n\u001b[91m❌ ERROR: %s\u001b[0m\n", err)
			continue
		}

		// STREAM COPY PATTERN (eino_assistant 2 approach)
		// Split stream: one for display, one for memory
		streamCopies := streamReader.Copy(2)

		// Background goroutine: collect messages for history
		fullMsgs := make([]*schema.Message, 0)
		go func() {
			defer streamCopies[1].Close()

			for {
				chunk, err := streamCopies[1].Recv()
				if err != nil {
					if err == io.EOF {
						break
					}
					return
				}
				fullMsgs = append(fullMsgs, chunk)
			}

			// Save to conversation after stream completes
			a.conversation = append(a.conversation, schema.UserMessage(userInput))
			if len(fullMsgs) > 0 {
				fullMsg, err := schema.ConcatMessages(fullMsgs)
				if err == nil {
					a.conversation = append(a.conversation, fullMsg)
				}
			}
		}()

		// Foreground: display to user with visual feedback
		var thinkingMode bool

		for {
			chunk, err := streamCopies[0].Recv()
			if err != nil {
				if err == io.EOF {
					break
				}
				fmt.Printf("\n\n\u001b[91m❌ ERROR: %s\u001b[0m\n", err)
				break
			}

			// Handle Gemini thinking tags (show in RED like Bill's reasoning!)
			content := chunk.Content
			if strings.Contains(content, "<think>") {
				thinkingMode = true
				content = strings.ReplaceAll(content, "<think>", "\n\u001b[91m[💭 Thinking]\u001b[0m\n")
			}
			if strings.Contains(content, "</think>") {
				thinkingMode = false
				content = strings.ReplaceAll(content, "</think>", "\n")
			}

			// Display content (thinking in RED, normal text in white)
			if content != "" {
				if thinkingMode {
					fmt.Printf("\u001b[91m%s\u001b[0m", content)
				} else {
					fmt.Print(content)
				}
			}
		}

		fmt.Print("\n")
	}

	return nil
}
