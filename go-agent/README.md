# Go OTEL Agent with eBPF - 00/using-cilium-ebpf-custom-tracec

Go-based LLM agent with eBPF kernel-level observability, sending traces through the OTEL Collector to Jaeger and Galileo. What we scaffolded for this brnach is a custom eBPF setup using cilium/ebpf directly - using our own trace.c and loader.

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
go-agent
├── bpf
│   └── trace.c               # eBPF C program - TCP connect tracepoint
├── cmd
│   └── agent
│       └── main.go           # Entry point - CLI flags, wires tracing + eBPF + agent
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

# eBPF C program - TCP connect tracepoint

yeah this get its own heading.

The structure you’re looking at:

```
go-agent
├── bpf
│   └── trace.c   # eBPF C program - TCP connect
```

comes from a **specific era in observability tooling** when projects started combining **Go user-space agents with eBPF programs running inside the Linux kernel**.

To understand it, the history goes roughly something like this.

---

# 1. The Original Problem (Pre-2015)

Before eBPF, monitoring tools had two main options:

### Option 1 — Modify application code

Example:

* Add instrumentation libraries
* Insert tracing calls
* Deploy agents

Example tools:

* New Relic
* Datadog
* Zipkin

Problem:

* Requires **changing application code**
* Language-specific agents
* Hard to instrument legacy systems

---

### Option 2 — Kernel tracing tools

Linux had tools like:

* `tcpdump`
* `perf`
* `strace`

Problem:

* Very **low level**
* Hard to extend
* High overhead

---

# 2. eBPF Changes Everything (~2014)

Linux introduced **Extended Berkeley Packet Filter (eBPF)**.

It allowed:

* small programs
* loaded into the **kernel**
* executed safely
* attached to kernel events

Examples:

* TCP connect
* syscalls
* disk I/O
* network packets

So now you could say:

> "Run this code every time a process calls `connect()`."

---

# 3. The Two-Part Architecture Emerges

Once eBPF appeared, observability systems started using a **split design**:

### Part 1 — Kernel program (C)

Runs in kernel using eBPF.

Example:

```
bpf/trace.c
```

Purpose:

* hook kernel events
* capture data
* push events to userspace

Example hook:

```
kprobe/tcp_v4_connect
```

---

### Part 2 — User-space agent (Go)

Written in Go.

Why Go?

* safe memory
* good concurrency
* easy networking
* widely used in cloud tooling

Example responsibilities:

* load eBPF program
* collect events
* export telemetry
* talk to backend APIs

---

# 4. Why `trace.c` Exists

Even if the project is written in Go, **the kernel program must be written in C**.

So projects end up with:

```
agent/
  main.go
bpf/
  trace.c
```

`trace.c` does things like:

```c
SEC("kprobe/tcp_v4_connect")
int trace_connect(struct pt_regs *ctx)
```

Meaning:

> When a TCP connection is created, run this code.

Then send data to Go via:

* **perf buffers**
* **ring buffers**
* **maps**

---

# 5. This Architecture Became Standard

Many modern observability tools use the same design.

Examples:

### Networking / security

* Cilium
* Falco

### Observability

* Pixie
* Parca

### Performance profiling

* Parca Agent

Typical layout:

```
agent (Go)
  loads eBPF

bpf (C)
  kernel programs
```

---

# 6. Where OpenTelemetry Fits

Recently, this architecture is being used for **automatic instrumentation**.

Example:

OpenTelemetry eBPF instrumentation projects.

Instead of:

```
app → SDK → spans
```

you get:

```
kernel → eBPF → agent → OTEL spans
```

Meaning:

> Tracing without modifying application code.

---

# 7. Why TCP Connect is Often the First Example

`tcp_connect` is commonly used because it is easy to trace.

It tells you:

* which process opened a connection
* destination IP
* destination port
* latency

This lets observability tools automatically detect:

* service dependencies
* database calls
* external APIs

---

# 8. Why the Folder is Called `bpf`

Almost every project uses the same convention:

```
bpf/
```

Because it contains:

* kernel programs
* compiled to BPF bytecode
* loaded by the agent

So:

```
trace.c
```

is simply:

> the kernel program that hooks TCP connections.

---

# The Big Picture

Your directory is basically this architecture:

```
Linux Kernel
   ↑
eBPF program (trace.c)
   ↑
Go agent
   ↑
Telemetry backend
```

---
