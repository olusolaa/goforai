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
go run examples/step1/main.go
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
go run examples/step2/main.go
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
# First, create the knowledge base:
go run ./cmd/indexing

# Then run the example:
go run examples/step3/main.go
```

**What it shows:**
1. Question: "Who are the keynote speakers?"
2. Retrieves relevant GopherCon documents
3. Adds documents to prompt context
4. Model answers using retrieved information

---

### Step 4: Tool Calling (ISOLATION)
**File:** `step4/main.go`

Demonstrates tool calling with a **single hardcoded question**. No chat loop - just the tool workflow.

**Concepts:**
- Defining tools with `InferTool`
- Tool metadata (name, description, parameters)
- Model requesting tool execution
- Executing tools and formatting results
- Sending results back to model
- Model using results in final answer

**Run:**
```bash
go run examples/step4/main.go
```

**What it shows:**
1. Question: "What is 25 multiplied by 17?"
2. Model identifies need for calculator tool
3. Model requests tool call with parameters
4. We execute the tool
5. We send result back
6. Model provides natural language answer

---

### Step 5: Full Integration 🚀 - Where Eino Shines
**File:** `step5/main.go`

**THIS is where Eino proves its value!** Bringing everything together:
- Terminal chat loop (from step 1 & 2)
- RAG retrieval (from step 3)  
- Tool calling (from step 4)
- **All orchestrated by Eino's graph + ReAct pattern**

**Why Eino here?** Steps 1-4 were simple enough for manual orchestration. But orchestrating RAG + Tools + Chat + Memory manually would require complex conditional logic, loops, and state management. Eino's graph makes it declarative and manageable.

**Concepts:**
- Eino graph composition (declarative node-based orchestration!)
- ReAct pattern for tool orchestration
- Automatic RAG retrieval for every query
- Tool calling via ReAct agent
- Conversation history management
- Complete streaming chat flow

**The Graph Architecture:**
```
User Input → Extract Query → RAG Retriever → Chat Template → ReAct Agent → Output
                ↓              (documents)         ↑
            Extract Vars  → → → → → → → → → → → → ↑
            (history, date)
```

**Run:**
```bash
# Make sure you have the knowledge base:
cd .. && go run ./cmd/indexing && cd example01

# Run the full agent:
go run step5/main.go
```

**Try asking:**
- "Who are the speakers at GopherCon Africa 2025?"
- "What is 25 multiplied by 17?"
- "Tell me about the talks, then calculate 100 + 42"

**The Magic:** The agent automatically:
- ✅ Retrieves relevant GopherCon docs for every query
- ✅ Uses calculator tool when math is needed
- ✅ Maintains conversation history
- ✅ Streams responses in real-time
- ✅ All orchestrated by Eino's graph!

---

### Step 6: Memory Persistence (Optional)
**File:** `step6/main.go` *(To be implemented)*

Adds conversation persistence across sessions.

**Concepts:**
- Persistent conversation memory
- Session management
- Loading/saving conversation history

---

## 🎓 For Your GopherCon Africa Talk

### Presentation Strategy

**DON'T show:**
- Steps 1-2 in detail (basics everyone knows)
- Internal Eino plumbing
- Environment setup (boring!)

**DO show:**
1. **Start with Step 3** - "Let's see RAG in action!"
   - Run it live, show document retrieval
   - Explain why RAG matters

2. **Move to Step 4** - "Now let's see tool calling!"
   - Run it live, show the workflow
   - Explain the back-and-forth

3. **Step 5 is your finale** - "Now watch them work together!"
   - Live demo of full agent
   - Show it handling both RAG and tool questions
   - Show tool chaining

4. **Quick diff walkthrough**
   ```bash
   # Show code differences
   diff step3/main.go step5/main.go
   ```

### Time Management (45 min talk)
- **Intro (5 min):** Why AI agents? Why Eino?
- **RAG Demo - Step 3 (10 min):** Run + explain
- **Tools Demo - Step 4 (10 min):** Run + explain
- **Integration - Step 5 (15 min):** Run + code walkthrough
- **Q&A (5 min)**

---

## 🛠️ Prerequisites

### Environment Variables
```bash
export GEMINI_API_KEY="your-api-key"
```

### Create Knowledge Base
```bash
go run ./cmd/indexing
```

This creates `data/chromem.gob` with GopherCon Africa 2025 information.

---

## 📖 Further Reading

- [Eino Documentation](https://github.com/cloudwego/eino)
- [Bill Kennedy's AI Training](https://github.com/ardanlabs/ai-training)
- [ReAct Pattern Paper](https://arxiv.org/abs/2210.03629)

---

## 🙏 Credits

This teaching approach is inspired by Bill Kennedy's excellent progressive examples at [Ardan Labs](https://github.com/ardanlabs/ai-training).

