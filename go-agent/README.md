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

## Prerequisites

- Go 1.23+
- Docker (for OTEL Collector + Jaeger — uses root docker-compose)
- `OPENAI_API_KEY` environment variable
- For eBPF: Linux kernel 5.8+, clang, bpftool, root privileges

## Quick Start

```bash
# Start infra from repo root
npm run infra:up

# Run without eBPF (no root needed)
cd go-agent
make run-no-ebpf

# Run with eBPF (requires root)
make run
```

## Project Structure

```
go-agent/
├── cmd/agent/          Entry point
├── internal/
│   ├── agent/          LLM agent logic (OpenAI)
│   ├── ebpf/           eBPF loader + event reader
│   ├── tracing/        OTEL SDK setup
│   └── tools/          Agent tool implementations
├── bpf/                eBPF C programs
└── Makefile            Build targets
```
