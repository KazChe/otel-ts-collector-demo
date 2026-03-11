// SPDX-License-Identifier: GPL-2.0
// eBPF program for kernel-level observability (Phase 1)
//
// Traces outbound TCP connect() syscalls so the OTEL agent can emit
// network-level spans alongside LLM call spans.

#include <linux/bpf.h>
#include <bpf/bpf_helpers.h>

struct event {
    __u32 pid;
    __u32 uid;
    __u16 dport;
    __u32 daddr;
    __u64 timestamp_ns;
};

// Ring buffer for sending events to userspace
struct {
    __uint(type, BPF_MAP_TYPE_RINGBUF);
    __uint(max_entries, 256 * 1024);
} events SEC(".maps");

// TODO: Attach to sys_enter_connect tracepoint
// SEC("tracepoint/syscalls/sys_enter_connect")
// int trace_connect(struct trace_event_raw_sys_enter *ctx) {
//     struct event *e = bpf_ringbuf_reserve(&events, sizeof(*e), 0);
//     if (!e) return 0;
//
//     e->pid = bpf_get_current_pid_tgid() >> 32;
//     e->uid = bpf_get_current_uid_gid();
//     e->timestamp_ns = bpf_ktime_get_ns();
//
//     // TODO: Extract destination address and port from sockaddr
//
//     bpf_ringbuf_submit(e, 0);
//     return 0;
// }

char LICENSE[] SEC("license") = "GPL";
