# Building an AI Agent from Scratch with Eino

This directory contains a progressive tutorial for building a production-ready AI agent using the [Eino framework](https://github.com/cloudwego/eino).

## 🎯 Teaching Strategy

Inspired by Bill Kennedy's approach at GopherCon UK, each step is **independently runnable** and teaches **one clear concept**. Complex features (RAG, tools) are first shown **in isolation**, then integrated into the full agent.

## 📚 The Steps

### Step 1: Basic Chat
**File:** `step1/main.go`

The simplest possible Eino agent - just a terminal chat with Gemini.

**Concepts:**
- Creating a basic Eino ChatModel
- Managing conversation history
- User input loop

**Run:**
```bash
# From the repository root:
make step1
```

---

### Step 2: System Prompt & Formatting
**File:** `step2/main.go`

Adds professional UX with system prompts, color-coded output, and streaming responses.

**Concepts:**
- System prompts to guide agent behavior
- Terminal color codes for better UX
- Streaming responses with `Stream()`
- Collecting and concatenating message chunks

**Run:**
```bash
# From the repository root:
make step2
```

**What changed from Step 1:**
- ✅ Added system prompt
- ✅ Color-coded terminal output (blue=user, yellow=model)
- ✅ Streaming responses instead of blocking
- ✅ Better error handling

**Note:** Steps 1-2 use direct component calls (manual orchestration). This keeps things simple and clear while teaching the concepts. **Step 5** will show where Eino's graph orchestration becomes essential.

---

### Step 3: RAG Retrieval (ISOLATION)
**File:** `step3/main.go`

Demonstrates RAG (Retrieval Augmented Generation) with a **single hardcoded question**. No chat loop - just the RAG workflow.

**Concepts:**
- Loading ChromemDB vector database
- Embedding queries with Gemini
- Retrieving relevant documents
- Augmenting prompts with retrieved context
- How RAG improves answer quality

**Run:**
```bash
# From the repository root:
make step3
# (automatically runs 'make setup' if knowledge base doesn't exist)
```

**What it shows:**
1. Question: "Who are the keynote speakers?"
2. Retrieves relevant GopherCon documents
3. Adds documents to prompt context
4. Model answers using retrieved information

---

### Step 4: RAG + Tools Integration
**File:** `step4/main.go`

Combines RAG retrieval with tool calling in an interactive chat loop using Eino's ReAct agent.

**Concepts:**
- Defining tools with `InferTool`
- Tool metadata (name, description, parameters)
- ReAct agent for intelligent tool orchestration
- Combining RAG retrieval with tool calling
- Interactive streaming chat with both capabilities

**Run:**
```bash
# From the repository root:
make step4
# Optional: export TAVILY_API_KEY="your-tavily-key" for Tavily search
```

**What it demonstrates:**
- RAG retrieval for GopherCon Africa questions
- Internet search tool (Tavily) for current events
- ReAct agent deciding when to use which capability
- Streaming responses with spinner UX
- Full conversation history management

---

### Step 5: Production-Ready Agent 🚀 - Clean Architecture
**File:** `step5/main.go` (with `agent/` and `ui/` packages)

**A production-grade agent with clean architecture!** This step demonstrates:
- **Proper Go project structure** with separated concerns
- **Eino graph orchestration** for complex workflows
- **Multiple tools** working together seamlessly
- **Professional UI** with thinking indicators and error handling

**Why Step 5 matters:** This shows how to build a **real production agent**, not just a demo. The code is organized, testable, and maintainable.

**Concepts:**
- Clean architecture (agent, UI, tools separation)
- Eino graph composition with type-safe nodes
- ReAct agent with multiple tools
- Callback handlers for UI updates
- Streaming with thinking mode visualization
- Dependency injection pattern

**The Graph Architecture:**
```
User Input → Extract Variables → Chat Template → ReAct Agent → Output
              (query, history, date)      ↓
                                    (system prompt)
                                          ↓
                                    [Multiple Tools]
                                    - RAG (GopherCon KB)
                                    - Web Search (Tavily/DDG)
                                    - Read File
                                    - Search Files
                                    - Edit File
                                    - Git Clone
```

**Project Structure:**
```
step5/
├── main.go           # Entry point & dependency wiring
├── agent/
│   ├── agent.go      # Agent orchestration & conversation loop
│   ├── graph.go      # Eino graph definition
│   └── tools.go      # Tool setup & configuration
└── ui/
    └── ui.go         # Terminal UI with callbacks
```

**Run:**
```bash
# From the repository root:
make step5
# (automatically runs 'make setup' if knowledge base doesn't exist)
```

**Try asking:**
- "Who are the speakers at GopherCon Africa 2025?" (uses RAG tool)
- "What's the latest news about Go 1.23?" (uses web search)
- "Search for main.go files in this project" (uses search files tool)
- "Read the README.md file" (uses read file tool)

**The Magic:** The agent:
- ✅ Uses Eino's graph for declarative orchestration
- ✅ Intelligently selects the right tool for each query
- ✅ Shows "thinking" process in real-time
- ✅ Maintains conversation history across turns
- ✅ Gracefully handles tool failures with fallbacks
- ✅ Clean, maintainable, production-ready code!

---

### Step 6: Memory Persistence (Optional)
**File:** `step6/main.go` *(To be implemented)*

Adds conversation persistence across sessions.

**Concepts:**
- Persistent conversation memory
- Session management
- Loading/saving conversation history

---

## 🛠️ Prerequisites

### Environment Variables
```bash
# Required for all steps
export GEMINI_API_KEY="your-api-key"

# Optional for steps 4 & 5 (web search)
# If not set, DuckDuckGo will be used as fallback
export TAVILY_API_KEY="your-tavily-key"
```

### Quick Start
```bash
# 1. Clone the repository
git clone https://github.com/olusolaa/goforai
cd goforai

# 2. Set up environment
export GEMINI_API_KEY="your-api-key"
# Optional: export TAVILY_API_KEY="your-tavily-key"

# 3. Create knowledge base (required for steps 3-5)
make setup

# 4. Run any step
make step1  # Basic chat
make step2  # With formatting
make step3  # RAG (auto-runs setup if needed)
make step4  # RAG + Tools
make step5  # Production agent

# See all commands
make help
```

---

## 📖 Further Reading

- [Eino Documentation](https://github.com/cloudwego/eino)
- [Bill Kennedy's AI Training](https://github.com/ardanlabs/ai-training)
- [ReAct Pattern Paper](https://arxiv.org/abs/2210.03629)

---

## 🙏 Credits

This teaching approach is inspired by Bill Kennedy's excellent progressive examples at [Ardan Labs](https://github.com/ardanlabs/ai-training).

