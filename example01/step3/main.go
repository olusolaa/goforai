// Step 3: Adding Knowledge (RAG) to Our Interactive Agent
//
// This is a direct evolution of Step 2. We take the polished, streaming, interactive
// chat agent and add RAG (Retrieval-Augmented Generation) capabilities. The agent
// will retrieve relevant documents from a vector database and use them to provide
// factual, context-aware answers. We maintain all the UX polish from Step 2.
//
// The narrative focuses on how Eino's components (Retriever + ChatTemplate)
// compose naturally, and why Go is exceptionally well-suited for the
// performance-critical task of retrieval.

package main

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"log"
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
	"github.com/cloudwego/eino/schema"
	"github.com/olusolaa/goforai/foundation/chromemdb"
)

// ---
// Step 1: The Orchestrator (Evolving for RAG)
// ---

func main() {
	if err := run(context.Background()); err != nil {
		log.Fatal(err)
	}
}

func run(ctx context.Context) error {
	// ********* NEW: Centralized AI client creation for model and embedder. *******
	clients, err := newAIClients(ctx)
	if err != nil {
		return err
	}

	// ************* NEW: We construct our retriever, the core of RAG. *************
	ragRetriever, err := newRetriever(ctx, clients.embedder) //This component is responsible for finding relevant information.
	if err != nil {
		return err
	}

	// ************ NEW: Create a chat template for RAG prompt formatting. *********
	ragTemplate, err := newRAGTemplate()
	if err != nil {
		return err
	}

	// ************ CHANGED: Create an agent with the new RAG components. **********
	agent := NewAgent(clients.chatModel, ragRetriever, ragTemplate, os.Stdin, os.Stdout)
	return agent.Run(ctx)
}

// ---
// Step 2: The Core Logic - A RAG-Powered Agent
// ---

// ******** CHANGED: System prompt is now specific to our RAG knowledge base. *******
const systemPrompt = `You are an assistant with access to a knowledge base about GopherCon Africa 2024. Use the provided context to answer questions accurately.`

// ****** CHANGED: Our Agent struct now includes retriever and template dependencies. ******

// composition adding new tools

type Agent struct {
	model     model.ToolCallingChatModel
	retriever retriever.Retriever // Finds relevant documents. chromem-go, go native (745 stars)
	template  prompt.ChatTemplate // Formats prompts with context.
	scanner   *bufio.Scanner
	out       io.Writer
}

// ********* CHANGED: The constructor now accepts the new RAG components. **********

func NewAgent(m model.ToolCallingChatModel, r retriever.Retriever, t prompt.ChatTemplate, in io.Reader, out io.Writer) *Agent {
	return &Agent{
		model:     m,
		retriever: r,
		template:  t,
		scanner:   bufio.NewScanner(in),
		out:       out,
	}
}

// Run is a direct evolution of Step 2, but now performs RAG retrieval before each query.
func (a *Agent) Run(ctx context.Context) error {
	conversation := []*schema.Message{schema.SystemMessage(systemPrompt)}
	fmt.Fprintf(a.out, "\nChat with a RAG-powered agent (use 'ctrl-c' to quit)\n")

	for {
		fmt.Fprintf(a.out, "%s\nYou%s: ", colorBlue, colorReset)
		if !a.scanner.Scan() {
			break
		}
		userInput := a.scanner.Text()
		if userInput == "" {
			continue
		}

		// ********* NEW: Retrieve relevant documents for the user's question. *********
		// doing heavy vector math. In Go, this is fast, compiled code running without a GIL, and
		// can be easily parallelized.
		docs, err := a.retriever.Retrieve(ctx, userInput)
		if err != nil {
			fmt.Fprintf(a.out, "\n\n%sERROR: %s%s\n\n", colorRed, err, colorReset)
			continue
		}

		// ****** NEW: Format the prompt with retrieved context using a ChatTemplate. ******
		var sb strings.Builder
		for i, doc := range docs {
			if i > 0 {
				sb.WriteString("\n\n")
			}
			sb.WriteString(fmt.Sprintf("Document %d:\n%s", i+1, doc.Content))
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

		streamReader, err := a.model.Stream(ctx, conversation)
		if err != nil {
			fmt.Fprintf(a.out, "\n\n%sERROR: %s%s\n\n", colorRed, err, colorReset)
			continue
		}

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
// Step 3: Centralized Factories for Dependencies
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

// ******** NEW: A struct to hold all our AI clients for efficient creation. *******
type aiClients struct {
	chatModel model.ToolCallingChatModel
	embedder  embedding.Embedder
}

// ******** NEW: A single factory to create all AI clients from one base client. ********
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

// ********* NEW: A factory specifically for our vector database retriever. *********
func newRetriever(ctx context.Context, embedder embedding.Embedder) (retriever.Retriever, error) {
	return chromemdb.New(ctx, "gophercon-knowledge", embedder,
		chromemdb.WithDBPath(dbPath),
		chromemdb.WithTopK(3),
	)
}

// **************** NEW: A factory for the RAG chat template. ******************
func newRAGTemplate() (prompt.ChatTemplate, error) {
	return prompt.FromMessages(
		schema.FString,
		schema.SystemMessage(systemPrompt),
		schema.UserMessage(`Based ONLY on the following context, answer the question.

Context:
{context}

Question: {question}`),
	), nil
}
