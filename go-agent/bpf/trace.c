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

// Tracepoint args for syscalls/sys_enter_connect
// See /sys/kernel/tracing/events/syscalls/sys_enter_connect/format
struct sys_enter_connect_args {
    unsigned long long unused;
    long syscall_nr;
    unsigned long fd;
    unsigned long uservaddr;
    unsigned long addrlen;
};

SEC("tracepoint/syscalls/sys_enter_connect")
int trace_connect(struct sys_enter_connect_args *ctx) {
    struct event *e = bpf_ringbuf_reserve(&events, sizeof(*e), 0);
    if (!e)
        return 0;

    e->pid = bpf_get_current_pid_tgid() >> 32;
    e->uid = bpf_get_current_uid_gid();
    e->timestamp_ns = bpf_ktime_get_ns();

    // Read sa_family (first 2 bytes of sockaddr)
    __u16 sa_family = 0;
    bpf_probe_read_user(&sa_family, sizeof(sa_family), (void *)ctx->uservaddr);

    // Only handle AF_INET (IPv4) for now
    if (sa_family == 2) { // AF_INET
        // sockaddr_in layout: sa_family(2) + sin_port(2) + sin_addr(4)
        bpf_probe_read_user(&e->dport, sizeof(e->dport), (void *)(ctx->uservaddr + 2));
        bpf_probe_read_user(&e->daddr, sizeof(e->daddr), (void *)(ctx->uservaddr + 4));
    }

    bpf_ringbuf_submit(e, 0);
    return 0;
}

char LICENSE[] SEC("license") = "GPL";
