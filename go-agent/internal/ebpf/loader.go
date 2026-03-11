package ebpf

import (
	"context"
	"fmt"
)

// Start loads the eBPF program and begins emitting kernel-level events as OTEL spans.
// Returns a cleanup function to detach the eBPF program.
func Start(ctx context.Context) (func(), error) {
	// TODO: Phase 1 — Observability layer
	// 1. Load compiled eBPF object (bpf/trace.o)
	// 2. Attach to tracepoints (e.g., syscalls:sys_enter_connect for TCP connections)
	// 3. Read events from perf/ring buffer
	// 4. Convert events to OTEL spans with attributes:
	//    - net.peer.name, net.peer.port (for outbound connections)
	//    - syscall name, latency, return code
	// 5. Parent spans under the active agent trace context

	fmt.Println("[ebpf] TODO: load eBPF program and attach to tracepoints")

	cleanup := func() {
		fmt.Println("[ebpf] detaching eBPF programs")
	}

	return cleanup, nil
}
