// Step 2: Adding Production Features & Superior UX
//
// This is a direct evolution of Step 1. We take the basic agent and add features
// you'd expect in a polished application. The narrative is about moving from a
// simple tool to a polished application, showcasing how Go's core features make
// this evolution straightforward and robust.

package main

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"time"

	geminiModel "github.com/cloudwego/eino-ext/components/model/gemini"
	"google.golang.org/genai"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
)

// ---
// Foundation (Unchanged from Step 1)
// ---
func main() {
	if err := run(context.Background()); err != nil {
		log.Fatal(err)
	}
}

func run(ctx context.Context) error {
	chatModel, err := newChatModel(ctx)
	if err != nil {
		return fmt.Errorf("failed to create chat model: %w", err)
	}
	agent := NewAgent(chatModel, os.Stdin, os.Stdout)
	return agent.Run(ctx)
}

// ---
// The Core Logic - Evolving for Production
// ---

// ******************* NEW: Constants for UI enhancement. *********************
const (
	colorBlue   = "\u001b[94m"
	colorYellow = "\u001b[93m"
	colorRed    = "\u001b[91m"
	colorReset  = "\u001b[0m"
)

// ************** NEW: The system prompt for behavioral control. **************
const systemPrompt = `You are a helpful coding assistant for Go. Never provide code examples in Python. Also never be verboose be very concise.`

type Agent struct {
	model   model.ToolCallingChatModel
	scanner *bufio.Scanner
	out     io.Writer
}

func NewAgent(m model.ToolCallingChatModel, in io.Reader, out io.Writer) *Agent {
	return &Agent{model: m, scanner: bufio.NewScanner(in), out: out}
}

// Run is where we see the most significant evolution from a basic script to a
// sophisticated, user-friendly application.
func (a *Agent) Run(ctx context.Context) error {
	// *********** CHANGED: We now initialize state with a system prompt. ***********
	conversation := []*schema.Message{schema.SystemMessage(systemPrompt)}
	fmt.Fprintf(a.out, "\nChat with %s (use 'ctrl-c' to quit)\n", chatModelName)
	for {
		fmt.Fprintf(a.out, "%s\nYou%s: ", colorBlue, colorReset)
		if !a.scanner.Scan() {
			break
		}
		userInput := a.scanner.Text()
		if userInput == "" {
			continue
		}
		conversation = append(conversation, schema.UserMessage(userInput))
		fmt.Fprintf(a.out, "%s\n%s%s: ", colorYellow, chatModelName, colorReset)
		// ************ CHANGED: We've switched to a streaming model call. *************
		streamReader, err := a.model.Stream(ctx, conversation)
		if err != nil {
			fmt.Fprintf(a.out, "\n\n%sERROR: %s%s\n\n", colorRed, err, colorReset)
			continue
		}

		// ***************** NEW: Concurrent spinner for superior UX. ******************
		done := make(chan struct{})
		go a.showSpinner(done)

		// ************** NEW: The idiomatic Go stream consumption loop. ***************
		var chunks []*schema.Message
		firstChunk := true
		for {
			chunk, err := streamReader.Recv()
			if err != nil {
				if firstChunk {
					close(done)
				}
				if err == io.EOF { //`io.EOF` is not an exception, it's an expected value.
					break
				}
				fmt.Fprintf(a.out, "\n\n%sERROR: %s%s\n\n", colorRed, err, colorReset)
				break
			}
			if firstChunk {
				close(done)
				time.Sleep(10 * time.Millisecond) // Allow spinner to erase.
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

// ************* NEW: Helper method for the concurrent spinner. ****************
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
// Dependencies and Configuration (Unchanged)
// ---
const chatModelName = "gemini-2.5-flash"

func newChatModel(ctx context.Context) (model.ToolCallingChatModel, error) {
	apiKey := os.Getenv("GEMINI_API_KEY")
	if apiKey == "" {
		return nil, errors.New("GEMINI_API_KEY environment variable not set")
	}
	client, err := genai.NewClient(ctx, &genai.ClientConfig{APIKey: apiKey})
	if err != nil {
		return nil, fmt.Errorf("failed to create gemini client: %w", err)
	}
	config := &geminiModel.Config{Client: client, Model: chatModelName}
	return geminiModel.NewChatModel(ctx, config)
}
