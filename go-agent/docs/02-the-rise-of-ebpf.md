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

## Compiling trace.c

Must be done inside the OrbStack VM (eBPF needs Linux):

```bash
orb -m ebpf-dev
cd /Users/kam/development/OTEL/otel-ts-collector-demo/go-agent
clang -O2 -g -target bpf -I/usr/include/aarch64-linux-gnu -c bpf/trace.c -o bpf/trace.o
```

## Running with eBPF

```bash
# Inside OrbStack VM, from go-agent/
sudo make run
```

This requires:
- Linux kernel 5.8+ (OrbStack has 6.17)
- Root privileges (for loading eBPF programs)
- `bpf/trace.o` compiled (see above)
- OTEL Collector + Jaeger running (`npm run infra:up` from repo root)

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
| `go.mod` / `go.sum` | Added cilium/ebpf v0.21.0 |
