// Step 3: RAG Retrieval in ISOLATION
//
// This example demonstrates the RAG (Retrieval Augmented Generation) workflow
// using a single hardcoded question. This isolates the RAG mechanics so you can
// see exactly what happens:
//  1. User asks a question
//  2. System retrieves relevant documents from vector database
//  3. Documents are added to context
//  4. Model answers based on retrieved documents
//
// # Running the example:
//
//	go run examples/step3/main.go
//
// # Requirements:
//
//	GEMINI_API_KEY environment variable must be set
//	data/chromem.gob must exist (run: go run ./cmd/indexing)

package main

import (
	"context"
	"fmt"
	"log"
	"strings"

	"github.com/cloudwego/eino/schema"
	"github.com/olusolaa/goforai/foundation"
	"github.com/olusolaa/goforai/foundation/chromemdb"
)

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	ctx := context.Background()

	// Hardcoded question to demonstrate RAG
	question := "Who are the keynote speakers at GopherCon Africa 2025?"

	fmt.Printf("Question:\n\n%s\n", question)

	// STEP 1: LOAD THE VECTOR DATABASE
	fmt.Println("\n" + strings.Repeat("=", 80))
	fmt.Println("STEP 1: Loading ChromemDB Vector Database")
	fmt.Println(strings.Repeat("=", 80))

	// Use foundation retriever!
	embedder, err := foundation.NewEmbedder(ctx)
	if err != nil {
		return fmt.Errorf("failed to create embedder: %w", err)
	}

	chromemRetriever, err := chromemdb.New(ctx, "example01", embedder,
		chromemdb.WithDBPath("./data/chromem.gob"),
		chromemdb.WithTopK(3))
	if err != nil {
		return fmt.Errorf("failed to create retriever: %w", err)
	}

	// STEP 2: RETRIEVE RELEVANT DOCUMENTS
	fmt.Println("\n" + strings.Repeat("=", 80))
	fmt.Println("STEP 2: Retrieving Relevant Documents")
	fmt.Println(strings.Repeat("=", 80))

	docs, err := chromemRetriever.Retrieve(ctx, question)
	if err != nil {
		return fmt.Errorf("failed to retrieve documents: %w", err)
	}

	fmt.Printf("\nRetrieved %d documents:\n\n", len(docs))
	for i, doc := range docs {
		fmt.Printf("\u001b[92mDocument %d\u001b[0m:\n", i+1)
		content := strings.TrimSpace(doc.Content)
		if len(content) > 200 {
			content = content[:200] + "..."
		}
		fmt.Printf("  %s\n\n", content)
	}

	// STEP 3: CREATE AUGMENTED PROMPT WITH RETRIEVED DOCUMENTS
	fmt.Println(strings.Repeat("=", 80))
	fmt.Println("STEP 3: Creating Augmented Prompt with Retrieved Context")
	fmt.Println(strings.Repeat("=", 80))

	docsText := ""
	for _, doc := range docs {
		docsText += doc.Content + "\n\n"
	}

	augmentedPrompt := fmt.Sprintf(`Based on the following information, answer the question.

Context Documents:
==== doc start ====
%s
==== doc end ====

Question: %s`, docsText, question)

	fmt.Printf("\n\u001b[93mAugmented Prompt Length: %d characters\u001b[0m\n", len(augmentedPrompt))

	// STEP 4: GET MODEL RESPONSE WITH AUGMENTED CONTEXT
	fmt.Println("\n" + strings.Repeat("=", 80))
	fmt.Println("STEP 4: Generating Response with Retrieved Context")
	fmt.Println(strings.Repeat("=", 80) + "\n")

	// Use foundation component!
	chatModel, err := foundation.NewChatModel(ctx)
	if err != nil {
		return fmt.Errorf("failed to create chat model: %w", err)
	}

	conversation := []*schema.Message{
		schema.SystemMessage("You are a helpful assistant. Answer questions based on the provided context documents."),
		schema.UserMessage(augmentedPrompt),
	}

	fmt.Printf("\u001b[93m%s\u001b[0m: ", foundation.ChatModelName)

	resp, err := chatModel.Generate(ctx, conversation)
	if err != nil {
		return fmt.Errorf("failed to generate response: %w", err)
	}

	fmt.Printf("\n%s\n", resp.Content)

	fmt.Println("\n" + strings.Repeat("=", 80))
	fmt.Println("✅ RAG workflow complete! The model answered using retrieved documents.")
	fmt.Println(strings.Repeat("=", 80))

	return nil
}
