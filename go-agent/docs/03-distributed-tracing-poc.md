# 03/distributed-tracing-poc

This branch demonstrates distributed tracing across two services using OpenTelemetry. Service A (Python/LangGraph) starts a trace, calls Service B (TypeScript/Express), and the `traceparent` header propagates automatically — no manual header code needed.

## Architecture

```
User → POST /ask (:8000)
         ↓
  Service A (Python/LangGraph)
    GalileoSpanProcessor → Galileo (direct)
    OpenInference: LangGraphInstrumentor
    Auto-instruments: FastAPI + httpx
         ↓ traceparent header (automatic)
  Service B (TypeScript/Express :3000)
    OTEL auto-instruments: HTTP + Express
    → OTLP/HTTP → Collector → Galileo
         ↓
    Agent logic (invoke_agent → chat → tool → chat)

OTEL Collector (:4318)
  ├── Galileo (otlphttp — for TS spans)
  └── Debug (stdout)
```

## How traceparent propagation works

1. Service A receives `POST /ask` — FastAPI auto-instrumentation creates a server span with a new `trace_id`
2. LangGraph executes the graph — `call_ts_service` node uses `httpx` to call Service B
3. `opentelemetry-instrumentation-httpx` auto-injects the `traceparent` header: `00-<trace_id>-<span_id>-01`
4. Service B receives the request — `@opentelemetry/instrumentation-http` auto-extracts the `traceparent` header
5. All spans created in Service B inherit the same `trace_id`
6. Both services export spans — Galileo reconstructs the full parent-child tree

Zero manual header code. Both sides handled by OTEL SDK auto-instrumentation.

## How spans reach Galileo

| Service | Path to Galileo |
|---------|----------------|
| Python (Service A) | `GalileoSpanProcessor` — direct OTLP to Galileo, auth from env vars |
| TypeScript (Service B) | OTLP → Collector → `otlphttp/galileo` exporter |

## What changed

### New: `python-service/` (Service A)

| File | Purpose |
|------|---------|
| `app.py` | FastAPI + LangGraph graph (`call_ts_service` → `format_response`) |
| `tracing.py` | OTEL SDK with GalileoSpanProcessor + OpenInference LangGraphInstrumentor |
| `requirements.txt` | Python dependencies |
| `Dockerfile` | Container image |

### New: `src/server.ts` (Service B HTTP entry point)

Express server wrapping the existing agent logic. `POST /agent` calls `runConversationTurn`.

### Modified: `src/tracing.ts`

Added HTTP + Express auto-instrumentation via `registerInstrumentations()`. This enables automatic `traceparent` extraction from incoming requests. Service name is now configurable via `OTEL_SERVICE_NAME`.

### Modified: `src/agent.ts`

Exported `runConversationTurn` so `server.ts` can call it.

### Modified: `docker-compose.yml`

Removed Jaeger. Added `service-a-python` and `service-b-ts` containers.

### Modified: `otel-collector-config.yaml`

Removed Jaeger exporter. Pipeline now exports to Galileo + debug only.

## Quick start

```bash
# Start everything
docker compose up -d

# Send a request
curl -X POST http://localhost:8000/ask \
  -H 'Content-Type: application/json' \
  -d '{"question":"Find me a good restaurant"}'
```

## Local development (services outside Docker)

```bash
# Start just the collector
docker compose up -d otel-collector

# Terminal 1: Service B (TypeScript)
npm run start:server

# Terminal 2: Service A (Python)
cd python-service
pip install -r requirements.txt
uvicorn app:app --port 8000

# Test
curl -X POST http://localhost:8000/ask \
  -H 'Content-Type: application/json' \
  -d '{"question":"Find me a good restaurant"}'
```

## Verification in Galileo

1. Open Galileo console
2. Find traces for the project
3. A single trace should show spans from both services:
   - `python-langgraph-service`: FastAPI server span → LangGraph graph spans → httpx client span
   - `otel-ts-collector-demo`: Express server span → invoke_agent → chat → execute_tool → chat
4. The parent-child tree is fully reconstructed from the shared trace ID

## Dependencies added

### Python
| Package | Purpose |
|---------|---------|
| `galileo` | GalileoSpanProcessor for direct Galileo export |
| `langgraph` | Graph-based agent orchestration |
| `fastapi` + `uvicorn` | HTTP server |
| `httpx` | Async HTTP client for calling Service B |
| `opentelemetry-instrumentation-fastapi` | Auto-instrument inbound requests |
| `opentelemetry-instrumentation-httpx` | Auto-inject traceparent into outbound requests |
| `openinference-instrumentation-langgraph` | GenAI attributes on LangGraph spans |

### TypeScript (added)
| Package | Purpose |
|---------|---------|
| `express` | HTTP server |
| `@opentelemetry/instrumentation-http` | Auto-instrument HTTP (extracts traceparent) |
| `@opentelemetry/instrumentation-express` | Auto-instrument Express routes |
