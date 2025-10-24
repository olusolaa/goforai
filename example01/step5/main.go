// cmd/main.go
package main

import (
	"context"
	"log"
	"os"

	"github.com/olusolaa/goforai/example01/step5/agent"
	"github.com/olusolaa/goforai/example01/step5/ui"
)

func main() {
	// The main function now follows the classic Go pattern:
	// create dependencies, inject them, and run.
	if err := run(); err != nil {
		// Using log.Fatalf is more idiomatic for fatal errors at startup.
		log.Fatalf("❌ Application failed: %v", err)
	}
}

// run encapsulates the application's startup and execution logic.
func run() error {
	// Ensure the required API key is set, failing early if it's not.
	if os.Getenv("GEMINI_API_KEY") == "" {
		log.Fatal("GEMINI_API_KEY environment variable must be set")
	}

	ctx := context.Background()

	// 1. Initialize the UI component. It's a dependency for the agent.
	terminalUI := ui.New()

	// 2. Create the agent, injecting the UI.
	// This decouples the agent's logic from its presentation.
	gopherAgent, err := agent.New(ctx, terminalUI)
	if err != nil {
		return err // Error is already well-contextualized by agent.New
	}

	// 3. Start the agent's main loop.
	return gopherAgent.Run(ctx)
}
