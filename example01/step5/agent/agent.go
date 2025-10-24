// internal/agent/agent.go
package agent

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/schema"
	"github.com/olusolaa/goforai/example01/step5/ui"
)

// Agent orchestrates the Eino graph and manages the conversation state.
// It is decoupled from the UI, which is provided as a dependency.
type Agent struct {
	graph        compose.Runnable[*UserMessage, *schema.Message]
	ui           *ui.TerminalUI
	conversation []*schema.Message
}

// UserMessage defines the input structure for the agent's graph.
// It's a clear contract for what the graph expects.
type UserMessage struct {
	Query   string
	History []*schema.Message
}

// New creates and initializes a new Agent.
// It builds the Eino graph and sets up the initial state.
func New(ctx context.Context, ui *ui.TerminalUI) (*Agent, error) {
	graph, err := buildEinoGraph(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to build agent graph: %w", err)
	}

	return &Agent{
		graph:        graph,
		ui:           ui,
		conversation: make([]*schema.Message, 0),
	}, nil
}

// Run starts the main interactive loop for the agent.
func (a *Agent) Run(ctx context.Context) error {
	a.ui.DisplayWelcome()

	for {
		userInput, ok := a.ui.GetUserInput()
		if !ok || strings.ToLower(userInput) == "exit" || strings.ToLower(userInput) == "quit" {
			fmt.Println("\n👋 Goodbye!")
			return nil
		}
		if userInput == "" {
			continue
		}

		// Execute the agent's logic for a single turn.
		if err := a.executeTurn(ctx, userInput); err != nil {
			a.ui.DisplayError(err)
		}
	}
}

// executeTurn handles a single user query, from graph execution to response streaming.
func (a *Agent) executeTurn(ctx context.Context, userInput string) error {
	input := &UserMessage{
		Query:   userInput,
		History: a.conversation,
	}

	a.ui.DisplayBotPrompt()

	// The UI itself is the callback handler, cleanly connecting agent events to the UI.
	cbHandler := a.ui.Build()

	streamReader, err := a.graph.Stream(ctx, input, compose.WithCallbacks(cbHandler))
	if err != nil {
		return fmt.Errorf("graph execution failed: %w", err)
	}
	defer streamReader.Close()

	// Process the streaming response, updating the UI and conversation history concurrently.
	return a.processStream(streamReader, userInput)
}

// processStream handles the reading of the response stream.
// It collects chunks for history while updating the UI in real-time.
func (a *Agent) processStream(streamReader interface {
	Recv() (*schema.Message, error)
}, userInput string) error {
	var chunks []*schema.Message
	var thinkingMode bool

	for {
		chunk, err := streamReader.Recv()
		if err != nil {
			if err == io.EOF {
				break // End of stream
			}
			return fmt.Errorf("stream receive error: %w", err)
		}

		content := chunk.Content
		if strings.Contains(content, "<think>") {
			thinkingMode = true
			content = strings.ReplaceAll(content, "<think>", "\n")
		}
		if strings.Contains(content, "</think>") {
			thinkingMode = false
			content = strings.ReplaceAll(content, "</think>", "\n")
		}

		if content != "" {
			if thinkingMode {
				a.ui.DisplayThinking(content)
			} else {
				a.ui.DisplayStreamChunk(content)
			}
		}

		chunks = append(chunks, chunk)
	}

	// Concatenate all chunks and update conversation history
	var fullResponse *schema.Message
	if len(chunks) > 0 {
		fullResponse, _ = schema.ConcatMessages(chunks)
	}
	a.updateConversationHistory(userInput, fullResponse)

	fmt.Println()
	return nil
}

// updateConversationHistory appends the last user message and the full AI response
// to the conversation log for future context.
func (a *Agent) updateConversationHistory(userInput string, botResponse *schema.Message) {
	a.conversation = append(a.conversation, schema.UserMessage(userInput))
	if botResponse != nil {
		a.conversation = append(a.conversation, botResponse)
	}
}
