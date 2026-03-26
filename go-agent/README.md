# Go OTEL Agent with eBPF

Go-based LLM agent with eBPF kernel-level observability, sending traces through the OTEL Collector to Jaeger and Galileo.

## Architecture

```
Go Agent (LLM calls + eBPF kernel events)
         | OTLP/HTTP :4318
    OTEL Collector
    ├── Galileo (OTLP/HTTP)
    ├── Jaeger  (OTLP/gRPC)
    └── Debug   (stdout)
```

## Tutorial Iterations

| Branch | Doc | What it covers |
|--------|-----|----------------|
| `00/using-cilium-ebpf-custom-tracec` | [00 - Initial Setup](docs/00-initial-setup.md) | Scaffolding, OrbStack VM, eBPF toolchain, compiling trace.c |
| `01/implementation-of-stubs` | [01 - Implementation of Stubs](docs/01-implementation-of-stubs.md) | Real OpenAI calls, tool execution loop, OTEL spans |
| `02/the-rise-of-ebpf` | [02 - The Rise of eBPF](docs/02-the-rise-of-ebpf.md) | eBPF loader, tracepoint attach, ring buffer, kernel event spans |
| `03/distributed-tracing-poc` | [03 - Distributed Tracing POC](docs/03-distributed-tracing-poc.md) | Python/LangGraph + TS distributed tracing, GalileoSpanProcessor, traceparent propagation |

## Quick Start

```bash
# Start infra from repo root
npm run infra:up

# Run without eBPF (no root needed)
cd go-agent
make run-no-ebpf

# Run with eBPF (requires Linux + root)
make run
```

## Prerequisites

- Go 1.23+
- Docker (for OTEL Collector + Jaeger — uses root docker-compose)
- `OPENAI_API_KEY` environment variable

## Project Structure

```
go-agent
├── bpf
│   └── trace.c               # eBPF C program - TCP connect tracepoint
├── cmd
│   └── agent
│       └── main.go           # Entry point - CLI flags, wires tracing + eBPF + agent
├── docs                       # Tutorial docs per iteration
│   ├── 00-initial-setup.md
│   ├── 01-implementation-of-stubs.md
│   └── 02-the-rise-of-ebpf.md
└── internal
    ├── agent
    │   └── agent.go           # LLM conversation loop with OpenAI (real API calls)
    ├── ebpf
    │   └── loader.go          # Loads eBPF programs, reads kernel events as OTEL spans
    ├── tools
    │   └── tools.go           # Tool registry — callable functions available to the agent
    └── tracing
        └── tracing.go         # OTEL SDK setup — exporter, provider, resource config
```
