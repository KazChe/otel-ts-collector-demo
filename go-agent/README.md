# Go OTEL Agent with eBPF - 00/using-cilium-ebpf-custom-tracec

Go-based LLM agent with (extended Berkeley Packet Filter) eBPF kernel-level observability, sending traces through the OTEL Collector to Jaeger and Galileo. What we scaffolded for this brnach is a custom eBPF setup using cilium/ebpf directly - using our own trace.c and loader.

eBPF (Extended Berkeley Packet Filter) is a Linux kernel technology, originating from the 1992 "Berkeley Packet Filter" (BPF), that allows running sandboxed programs within the kernel to perform networking, security, and monitoring tasks. "Berkeley" refers to the University of California, Berkeley (specifically researchers at the Lawrence Berkeley National Laboratory).

The Creators: The original BPF was developed by Steven McCanne and Van Jacobson in 1992.

## cilium ebpf

ebpf-go is a pure Go library that provides utilities for loading, compiling, and debugging eBPF programs. It has minimal external dependencies and is intended to be used in long running processes.

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

>[!IMPORTANT]
>eBPF requires Linux kernel and root privileges to load programs into the kernel. If you're on macOS (Darwin - that heretic) eBPF won't work natively.

I chose [OrbStack](https://docs.orbstack.dev/#what-is-orbstack) for the first iterantion of this. Why, you ask and why is also what I wonder myself. Real reason is that I noticed they mention OrbStack `Starts in 2 seconds`.

Chose using the `brew install orbstack` and it went ... started in 2 seconds
- OrbStack [quick start](https://docs.orbstack.dev/quick-start)
- used `orb debug <container name or id>` to get their desktop app up
- you might have to:
```bash
 % orb login 
 Finish logging in at: https://orbstack.dev/app/auth/...
 ```

- orb cli:
```bash
~ % orb      
To use Docker:
    docker run ...
See "orb docker" for more info.

To create a Linux machine:
    orb create ubuntu
See "orb create --help" for supported distros and options.
```

## create a Linux machine

```bash
orb create ubuntu ebpf-dev 
orb -m ebpf-dev
```
once in your vm you can issue things like:
```bash
kam@ebpf-dev:/Users/kam$ orb list
NAME      STATE    DISTRO  VERSION   ARCH   SIZE      IP
----      -----    ------  -------   ----   ----      --
ebpf-dev  running  ubuntu  questing  arm64  716.5 MB  192.168.139.207
kam@ebpf-dev:/Users/kam$ uname -a
Linux ebpf-dev 6.17.8-orbstack-00308-g8f9c941121b1 #1 SMP PREEMPT Thu Nov 20 09:34:02 UTC 2025 aarch64 GNU/Linux
kam@ebpf-dev:/Users/kam$ 
```
here we see we're at `Kernel 6.17.8` - at this time `5.8` was minimum for eBPF.

## Next, install the eBPF toolchain inside the VM:

install the eBPF toolchain inside the VM 
eBPF toolchain is the set of tools needed to compile C into BPF bytecode and inspect/load it into the kernel

```bash
sudo apt update && sudo apt install -y clang llvm libbpf-dev bpftool
```
Then verify:
```bash
sudo bpftool feature probe kernel | head -20
```
we want to make sure that all the key ones are available 
— kprobe, tracepoint, perf_event. 
- The "skipping kernel config" warnings are cosmetic 
```bash
skipping kernel config, can't find correct file
Scanning system configuration...
bpf() syscall restricted to privileged users (admin can change)
Unable to retrieve JIT-compiler status
Unable to retrieve JIT hardening status
Unable to retrieve JIT kallsyms export status
Unable to retrieve global memory limit for JIT compiler for unprivileged users

Scanning system call availability...
bpf() syscall is available

Scanning eBPF program types...
eBPF program_type socket_filter is available
eBPF program_type kprobe is available
eBPF program_type sched_cls is available
eBPF program_type sched_act is available
eBPF program_type tracepoint is available
eBPF program_type xdp is available
eBPF program_type perf_event is available
eBPF program_type cgroup_skb is available
eBPF program_type cgroup_sock is available
```

now, our VM is eBPF-ready. You can now try compiling trace.c


```bash
sudo apt install -y gcc-aarch64-linux-gnu linux-libc-dev
```
This installed two packages:

- gcc-aarch64-linux-gnu - cross-compilation toolchain for ARM64, which brings along the architecture-specific system headers
- linux-libc-dev — Linux kernel headers that define types like __u32, __u16, __u64 (the ones used in your event struct in trace.c)

execute:
```bash
clang -O2 -g -target bpf -I/usr/include/aarch64-linux-gnu -c bpf/trace.c -o bpf/trace.o
```
Breaking it down:

| Flag                               | What it does                                                                      |
| ---------------------------------- | --------------------------------------------------------------------------------- |
| `-O2`                              | Optimize the output (eBPF verifier can reject unoptimized code)                   |
| `-g`                               | Include debug info (helps with `bpftool` inspection)                              |
| `-target bpf`                      | Compile to BPF bytecode instead of native machine code                            |
| `-I/usr/include/aarch64-linux-gnu` | Tell `clang` where to find the ARM64 kernel headers (the ones you just installed) |
| `-c`                               | Compile only, don't link (eBPF objects are loaded directly, not linked)           |
| `bpf/trace.c`                      | Your source file                                                                  |
| `-o bpf/trace.o`                   | Output: an ELF object containing BPF bytecode                                     |


The key insight: -target bpf means clang isn't producing ARM64 or x86 machine code — it's producing BPF bytecode that the Linux kernel's built-in BPF virtual machine executes. But it still needs the ARM64 headers to know the layout of kernel data structures on your architecture.

run:
```bash
ls -la bpf/trace.o
-rw-r--r-- 1 kam kam 3128 Mar 11 02:28 bpf/trace.o
```
That's your BPF bytecode sitting there waiting for the Go loader (internal/ebpf/loader.go) to load it into the kernel

That's it for this part. Next up, Real implementation of stubs.

- Go 1.23+
- Docker (for OTEL Collector + Jaeger — uses root docker-compose)
- `OPENAI_API_KEY` environment variable


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
