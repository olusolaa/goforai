// Step 1: Basic Eino Terminal Chat
//
// This example builds a simple AI chat agent, designed to be presented to a
// Go audience. The goal is to demonstrate how Go's core principles (simplicity,
// type safety, robust tooling) make it a superior choice for building production-ready
// AI systems, and how the Eino framework embraces these native Go idioms.

package main

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"os"

	geminiModel "github.com/cloudwego/eino-ext/components/model/gemini"
	"google.golang.org/genai"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
)

// ---
// Step 1: The Entry Point - Familiar Territory
// ---

func main() {
	if err := run(context.Background()); err != nil {
		log.Fatal(err)
	}
}

// ---
// Step 2: The Orchestrator - Clean Dependency Management
// ---

func run(ctx context.Context) error {
	chatModel, err := newChatModel(ctx)
	if err != nil {
		return fmt.Errorf("failed to create chat model: %w", err)
	}
	agent := NewAgent(chatModel, os.Stdin, os.Stdout)
	return agent.Run(ctx)
}

// ---
// Step 3: The Core Logic - An Idiomatic Go Worker
// ---

type Agent struct {
	model   model.ToolCallingChatModel // (eino 7.8k stars)
	scanner *bufio.Scanner
	out     io.Writer
}

func NewAgent(m model.ToolCallingChatModel, in io.Reader, out io.Writer) *Agent {
	return &Agent{
		model:   m,
		scanner: bufio.NewScanner(in),
		out:     out,
	}
}

func (a *Agent) Run(ctx context.Context) error {
	var conversation []*schema.Message
	fmt.Fprintf(a.out, "Chat with %s (use 'ctrl-c' to quit)\n", chatModelName)
	for {
		fmt.Fprint(a.out, "\nYou: ")
		if !a.scanner.Scan() {
			break
		}
		userInput := a.scanner.Text()
		if userInput == "" {
			continue
		}
		conversation = append(conversation, schema.UserMessage(userInput))

		resp, err := a.model.Generate(ctx, conversation)
		if err != nil {
			fmt.Fprintf(a.out, "\n\nERROR: %s\n\n", err)
			continue
		}
		fmt.Fprintf(a.out, "\n%s: %s\n", chatModelName, resp.Content)
		conversation = append(conversation, resp)
	}
	return a.scanner.Err()
}

// ---
// Step 4: The Factory - Abstracting Away the Details
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
