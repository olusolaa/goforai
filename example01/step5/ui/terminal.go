// internal/ui/terminal.go
package ui

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/cloudwego/eino/callbacks"
)

// TerminalUI handles all rendering and user interaction in the terminal.
// It implements the callbacks.Handler interface to react to agent events.
type TerminalUI struct {
	scanner         *bufio.Scanner
	spinner         *Spinner
	colorUser       func(a ...interface{}) string
	colorBot        func(a ...interface{}) string
	colorTool       func(a ...interface{}) string
	colorThinking   func(a ...interface{}) string
	colorSuccess    func(a ...interface{}) string
	colorError      func(a ...interface{}) string
	colorMuted      func(a ...interface{}) string
	colorHighlight  func(a ...interface{}) string
	activeToolMutex sync.Mutex
	activeToolName  string
}

// Color helper functions using ANSI codes
func colorize(color string, text ...interface{}) string {
	return fmt.Sprintf("%s%s\033[0m", color, fmt.Sprint(text...))
}

const (
	colorCodeBlue      = "\033[94;1m"
	colorCodeYellow    = "\033[93;1m"
	colorCodeGreen     = "\033[32m"
	colorCodeRed       = "\033[91;1m"
	colorCodeRedNormal = "\033[31m"
	colorCodeMuted     = "\033[2m"
	colorCodeCyan      = "\033[36m"
)

// New creates a new, configured TerminalUI instance.
func New() *TerminalUI {
	return &TerminalUI{
		scanner: bufio.NewScanner(os.Stdin),
		spinner: NewSpinner(100 * time.Millisecond),
		colorUser: func(a ...interface{}) string {
			return colorize(colorCodeBlue, a...)
		},
		colorBot: func(a ...interface{}) string {
			return colorize(colorCodeYellow, a...)
		},
		colorTool: func(a ...interface{}) string {
			return colorize(colorCodeGreen, a...)
		},
		colorThinking: func(a ...interface{}) string {
			return colorize(colorCodeRed, a...)
		},
		colorSuccess: func(a ...interface{}) string {
			return colorize(colorCodeGreen, a...)
		},
		colorError: func(a ...interface{}) string {
			return colorize(colorCodeRedNormal, a...)
		},
		colorMuted: func(a ...interface{}) string {
			return colorize(colorCodeMuted, a...)
		},
		colorHighlight: func(a ...interface{}) string {
			return colorize(colorCodeCyan, a...)
		},
	}
}

// DisplayWelcome prints the initial banner and instructions.
func (t *TerminalUI) DisplayWelcome() {
	border := "══════════════════════════════════════════════════════════════"
	fmt.Println(t.colorHighlight("╔" + border + "╗"))
	fmt.Println(t.colorHighlight("║") + "       🤖 Expert Go Coding Agent - Powered by Eino          " + t.colorHighlight("║"))
	fmt.Println(t.colorHighlight("╚" + border + "╝"))
	fmt.Println(t.colorMuted("\nTools: File Search/Read/Edit, Web Search, Git Clone, RAG | Type 'exit' to quit."))
	fmt.Println(t.colorMuted(strings.Repeat("─", 62)))
}

// GetUserInput prompts the user and returns their input.
func (t *TerminalUI) GetUserInput() (string, bool) {
	fmt.Printf("\n%s ", t.colorUser("You:"))
	if !t.scanner.Scan() {
		return "", false
	}
	return strings.TrimSpace(t.scanner.Text()), true
}

// DisplayBotPrompt shows the bot's name before it starts streaming.
func (t *TerminalUI) DisplayBotPrompt() {
	fmt.Printf("\n%s ", t.colorBot("Bot:"))
}

// DisplayThinking displays the model's reasoning process.
func (t *TerminalUI) DisplayThinking(content string) {
	fmt.Print(t.colorThinking(content))
}

// DisplayStreamChunk prints a part of the bot's response.
func (t *TerminalUI) DisplayStreamChunk(chunk string) {
	fmt.Print(chunk)
}

// DisplayError prints a formatted error message.
func (t *TerminalUI) DisplayError(err error) {
	fmt.Printf("\n%s %v\n", t.colorError("Error:"), err)
}

// OnStartFn is called when a component (like a tool) starts.
func (t *TerminalUI) OnStartFn(ctx context.Context, info *callbacks.RunInfo, input callbacks.CallbackInput) context.Context {
	if info.Component == "Tool" {
		t.activeToolMutex.Lock()
		defer t.activeToolMutex.Unlock()

		t.activeToolName = info.Name
		icon := getToolIcon(info.Name)
		msg := fmt.Sprintf(" %s %s", icon, t.colorTool(info.Name))
		go t.spinner.Start(msg)
	}
	return ctx
}

// OnEndFn is called when a component successfully finishes.
func (t *TerminalUI) OnEndFn(ctx context.Context, info *callbacks.RunInfo, output callbacks.CallbackOutput) context.Context {
	if info.Component == "Tool" {
		t.activeToolMutex.Lock()
		defer t.activeToolMutex.Unlock()

		if info.Name == t.activeToolName {
			t.spinner.Stop(t.colorSuccess("✓\n"))
			t.activeToolName = ""
		}
	}
	return ctx
}

// OnErrorFn is called when a component errors out.
func (t *TerminalUI) OnErrorFn(ctx context.Context, info *callbacks.RunInfo, err error) context.Context {
	if info.Component == "Tool" {
		t.activeToolMutex.Lock()
		defer t.activeToolMutex.Unlock()

		if info.Name == t.activeToolName {
			t.spinner.Stop(t.colorError("✗\n"))
			t.activeToolName = ""
			// Optionally print the specific error for debugging
			// fmt.Printf("%s\n", t.colorMuted(err.Error()))
		}
	}
	return ctx
}

// Build creates the callbacks.Handler from the UI methods.
func (t *TerminalUI) Build() callbacks.Handler {
	builder := callbacks.NewHandlerBuilder()
	builder.OnStartFn(t.OnStartFn)
	builder.OnEndFn(t.OnEndFn)
	builder.OnErrorFn(t.OnErrorFn)
	return builder.Build()
}

func getToolIcon(toolName string) string {
	switch toolName {
	case "search_files":
		return "🔍"
	case "read_file":
		return "📖"
	case "edit_go_file":
		return "✏️"
	case "search_internet", "tavily_search_results_json":
		return "🌐"
	case "gitclone":
		return "📥"
	case "rag_tool":
		return "📚"
	default:
		return "🛠️"
	}
}

// --- Spinner ---

// Spinner provides a simple terminal spinner.
type Spinner struct {
	ticker   *time.Ticker
	stopChan chan bool
	isActive bool
	mu       sync.Mutex
}

func NewSpinner(d time.Duration) *Spinner {
	return &Spinner{
		ticker:   time.NewTicker(d),
		stopChan: make(chan bool),
	}
}

func (s *Spinner) Start(message string) {
	s.mu.Lock()
	if s.isActive {
		s.mu.Unlock()
		return
	}
	s.isActive = true
	s.mu.Unlock()

	go func() {
		frames := `⠋⠙⠹⠸⠼⠴⠦⠧⠇⠏`
		i := 0
		for {
			select {
			case <-s.stopChan:
				return
			case <-s.ticker.C:
				fmt.Printf("\r%s%s ", message, string(frames[i%len(frames)]))
				i++
			}
		}
	}()
}

func (s *Spinner) Stop(finalMessage string) {
	s.mu.Lock()
	if !s.isActive {
		s.mu.Unlock()
		return
	}
	s.stopChan <- true
	s.isActive = false
	s.mu.Unlock()
	fmt.Printf("\r%s", finalMessage)
}
