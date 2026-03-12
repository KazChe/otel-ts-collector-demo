# 02/the-rise-of-ebpf

This branch brings the eBPF layer to life. The compiled BPF program now loads into the kernel, attaches to the `sys_enter_connect` tracepoint, and sends TCP connection events through a ring buffer to the Go agent — which converts them into OTEL spans.

## What changed

### bpf/trace.c — tracepoint hook activated

The previously commented-out tracepoint is now live:

```c
SEC("tracepoint/syscalls/sys_enter_connect")
int trace_connect(struct sys_enter_connect_args *ctx)
```

What it captures on every `connect()` syscall:
- **PID** — which process made the call
- **UID** — which user owns the process
- **Destination IP** — where the connection is going (IPv4, read from `sockaddr_in`)
- **Destination port** — which port (converted from network byte order)
- **Timestamp** — kernel monotonic clock in nanoseconds

Events are submitted to a **ring buffer** (`BPF_MAP_TYPE_RINGBUF`) for the Go agent to consume.

### internal/ebpf/loader.go — real eBPF loader

Replaced the stub with a full implementation using `cilium/ebpf`:

1. **Load** — reads `bpf/trace.o` from disk, parses the ELF with `LoadCollectionSpecFromReader`
2. **Attach** — hooks `trace_connect` to `tracepoint/syscalls/sys_enter_connect` via `link.Tracepoint`
3. **Read** — opens a `ringbuf.Reader` on the `events` map, spawns a goroutine to consume events
4. **Emit** — each event becomes an OTEL span with attributes:

| Attribute | Example | Source |
|-----------|---------|--------|
| `ebpf.event.type` | `tcp_connect` | hardcoded |
| `process.pid` | `1234` | `bpf_get_current_pid_tgid()` |
| `process.uid` | `1000` | `bpf_get_current_uid_gid()` |
| `net.peer.ip` | `142.250.80.46` | `sockaddr_in.sin_addr` |
| `net.peer.port` | `443` | `sockaddr_in.sin_port` (ntohs) |
| `ebpf.timestamp_ns` | `1234567890` | `bpf_ktime_get_ns()` |

5. **Cleanup** — closes ring buffer, detaches tracepoint, closes collection

### Makefile changes

- Removed `bpftool gen skeleton` step — we load `trace.o` at runtime via `os.ReadFile`, not via code generation
- `run` target now forwards `OPENAI_API_KEY` and `OTEL_EXPORTER_OTLP_ENDPOINT` through `sudo`

### Dependencies added

| Package | Version | Purpose |
|---------|---------|---------|
| `github.com/cilium/ebpf` | v0.21.0 | Load eBPF programs, attach tracepoints, read ring buffers |

## How it works end-to-end

```
1. Agent starts → loads trace.o into kernel
2. Kernel attaches trace_connect to sys_enter_connect
3. Agent calls OpenAI API → triggers TCP connect() to api.openai.com:443
4. Kernel fires trace_connect → event goes to ring buffer
5. Go goroutine reads event → creates OTEL span "tcp_connect"
6. Both LLM spans and eBPF spans flow through OTEL Collector → Jaeger
```

In Jaeger, you'll see the agent's LLM spans alongside kernel-level TCP connection spans — giving you visibility into what's happening at both the application and kernel layers.

## VM setup

eBPF requires Linux. We use OrbStack to run a Linux VM on macOS.

### First-time setup (if not done in 00/)

```bash
# Create and enter the VM
orb create ubuntu ebpf-dev
orb -m ebpf-dev

# Install eBPF toolchain
sudo apt update && sudo apt install -y clang llvm libbpf-dev bpftool
sudo apt install -y gcc-aarch64-linux-gnu linux-libc-dev

# Install Go
curl -LO https://go.dev/dl/go1.24.1.linux-arm64.tar.gz
sudo tar -C /usr/local -xzf go1.24.1.linux-arm64.tar.gz
export PATH=$PATH:/usr/local/go/bin

# Install make
sudo apt install -y make
```

### Compiling trace.c

Must be done inside the OrbStack VM:

```bash
orb -m ebpf-dev
cd /Users/kam/development/OTEL/otel-ts-collector-demo/go-agent
clang -O2 -g -target bpf -I/usr/include/aarch64-linux-gnu -c bpf/trace.c -o bpf/trace.o
```

No output means success. Verify with:
```bash
ls -la bpf/trace.o
file bpf/trace.o    # should show: ELF 64-bit LSB relocatable, eBPF
```

## Running with eBPF

Inside the OrbStack VM, from `go-agent/`:

```bash
# Source the .env for OPENAI_API_KEY
source ../.env

# Run with eBPF — sudo needs env vars forwarded explicitly
sudo env PATH=$PATH OPENAI_API_KEY=$OPENAI_API_KEY OTEL_EXPORTER_OTLP_ENDPOINT=host.internal:4318 make run
```

>[!IMPORTANT]
>`sudo` resets environment variables. The Makefile's `run` target forwards `OPENAI_API_KEY` and `OTEL_EXPORTER_OTLP_ENDPOINT` through the inner `sudo`, but `PATH` must include Go's bin directory for the build step.

>[!IMPORTANT]
>The OTEL Collector runs on your macOS host via Docker. From inside the VM, `localhost` points to the VM itself, not the host. Use `host.internal:4318` (OrbStack's alias for the macOS host) as the `OTEL_EXPORTER_OTLP_ENDPOINT`.

This requires:
- Linux kernel 5.8+ (OrbStack has 6.17)
- Root privileges (for loading eBPF programs)
- `bpf/trace.o` compiled (see above)
- OTEL Collector + Jaeger running on the host (`npm run infra:up` from repo root)

### What you'll see

```
[ebpf] attached to tracepoint/syscalls/sys_enter_connect
[ebpf] tcp_connect pid=4770 → 0.0.172.66:443          ← OpenAI API connection
[ebpf] tcp_connect pid=4770 → 0.0.0.250:53            ← DNS lookup
[chat] model=gpt-4o-mini tokens_in=115 tokens_out=27 finish=tool_calls
[tool] web_search → Results for "HNSW..."
[ebpf] tcp_connect pid=4770 → 0.0.0.250:4318          ← OTEL Collector export
[chat] model=gpt-4o-mini tokens_in=296 tokens_out=341 finish=stop

--- Agent Response ---
(HNSW explanation from OpenAI)
--- End ---

[ebpf] detaching eBPF programs
```

The eBPF layer catches every `connect()` syscall on the system — you'll see:
- `:53` — DNS lookups (resolving api.openai.com, host.internal)
- `:443` — HTTPS connections to OpenAI's API
- `:4318` — OTLP HTTP exports to the collector

## Running without eBPF

Still works on macOS — the agent gracefully skips eBPF:

```bash
make run-no-ebpf
```

## Files modified

| File | Change |
|------|--------|
| `bpf/trace.c` | Activated tracepoint hook, added sockaddr extraction |
| `internal/ebpf/loader.go` | Full implementation: load, attach, ring buffer, OTEL spans |
| `Makefile` | Removed bpftool skeleton step, forward env vars through sudo |
| `go.mod` / `go.sum` | Added cilium/ebpf v0.21.0 |
