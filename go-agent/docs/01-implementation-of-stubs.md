# 01/implementation-of-stubs

This branch brings the Go agent stubs to life with real OpenAI API calls and a functional tool-call loop. eBPF remains a stub — that's for `02/`.

## What changed

### agent.go — LLM conversation loop

The agent now runs a real multi-turn conversation with OpenAI:

1. Sends a system prompt + user message asking about **HNSW** (Hierarchical Navigable Small World)
2. OpenAI responds — if it returns `tool_calls`, the agent executes them via the tool registry
3. Tool results get sent back to the LLM for a final answer
4. The loop continues until the model returns a final response (no more tool calls)

Uses `gpt-4o-mini` model via `github.com/sashabaranov/go-openai`.

### OTEL span hierarchy

The agent produces spans that mirror the TypeScript agent's pattern:

```
invoke_agent (root span)
│   attributes: session.id, gen_ai.agent.name
│
├── chat (first LLM call)
│   attributes: gen_ai.request.model, gen_ai.provider.name
│   attributes: gen_ai.usage.input_tokens, gen_ai.usage.output_tokens
│   attributes: gen_ai.response.finish_reasons
│   events: gen_ai.user.message, gen_ai.choice
│
├── execute_tool (if tool_calls returned)
│   attributes: gen_ai.tool.name, gen_ai.tool.call.arguments
│   attributes: gen_ai.tool.call.result
│
└── chat (follow-up with tool results)
    (same attributes as first chat span)
```

Each conversation gets a `session.id` (UUID) on the root span for grouping in Galileo.

### tools.go — tool registry with OpenAI function definitions

- `web_search` — returns mock results about HNSW for the demo
- `get_time` — returns current date/time
- `OpenAITools()` method exports tool definitions in OpenAI's function calling format

### Dependencies added

| Package | Version | Purpose |
|---------|---------|---------|
| `github.com/sashabaranov/go-openai` | v1.41.2 | OpenAI chat completions |
| `github.com/google/uuid` | v1.6.0 | Session ID generation |

## Running it

```bash
# Start infra from repo root
npm run infra:up

# Run the agent (no eBPF needed)
cd go-agent
make run-no-ebpf
```

Requires `OPENAI_API_KEY` in the root `.env` file.

## What to check

- Jaeger at `http://localhost:16686` — look for `go-otel-agent` service
- You should see the span tree: `invoke_agent` → `chat` → `execute_tool` → `chat`
- Each `chat` span has token usage and model info
- Galileo receives the GenAI semantic convention attributes

## Files modified

| File | Change |
|------|--------|
| `internal/agent/agent.go` | Full LLM loop with tool-call handling + OTEL spans |
| `internal/tools/tools.go` | OpenAI function defs, mock HNSW results, get_time tool |
| `go.mod` / `go.sum` | Added go-openai and uuid dependencies |
