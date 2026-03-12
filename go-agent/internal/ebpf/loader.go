package ebpf

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"log"
	"net"
	"os"

	"github.com/cilium/ebpf"
	"github.com/cilium/ebpf/link"
	"github.com/cilium/ebpf/ringbuf"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

var tracer = otel.Tracer("go-otel-agent/ebpf")

// event must match the struct layout in bpf/trace.c
type event struct {
	PID         uint32
	UID         uint32
	Dport       uint16
	Daddr       uint32
	TimestampNs uint64
}

// Start loads the eBPF program and begins emitting kernel-level events as OTEL spans.
// Returns a cleanup function to detach the eBPF program.
func Start(ctx context.Context) (func(), error) {
	objBytes, err := os.ReadFile("bpf/trace.o")
	if err != nil {
		return nil, fmt.Errorf("reading bpf/trace.o: %w", err)
	}

	spec, err := ebpf.LoadCollectionSpecFromReader(bytes.NewReader(objBytes))
	if err != nil {
		return nil, fmt.Errorf("loading eBPF spec: %w", err)
	}

	coll, err := ebpf.NewCollection(spec)
	if err != nil {
		return nil, fmt.Errorf("creating eBPF collection: %w", err)
	}

	prog := coll.Programs["trace_connect"]
	if prog == nil {
		coll.Close()
		return nil, fmt.Errorf("eBPF program 'trace_connect' not found in collection")
	}

	tp, err := link.Tracepoint("syscalls", "sys_enter_connect", prog, nil)
	if err != nil {
		coll.Close()
		return nil, fmt.Errorf("attaching tracepoint: %w", err)
	}

	eventsMap := coll.Maps["events"]
	if eventsMap == nil {
		tp.Close()
		coll.Close()
		return nil, fmt.Errorf("ring buffer map 'events' not found")
	}

	rd, err := ringbuf.NewReader(eventsMap)
	if err != nil {
		tp.Close()
		coll.Close()
		return nil, fmt.Errorf("creating ring buffer reader: %w", err)
	}

	log.Println("[ebpf] attached to tracepoint/syscalls/sys_enter_connect")

	// Read events in background
	go func() {
		for {
			record, err := rd.Read()
			if err != nil {
				if errors.Is(err, ringbuf.ErrClosed) {
					return
				}
				log.Printf("[ebpf] read error: %v", err)
				continue
			}

			var ev event
			if err := binary.Read(bytes.NewReader(record.RawSample), binary.LittleEndian, &ev); err != nil {
				log.Printf("[ebpf] parse error: %v", err)
				continue
			}

			emitSpan(ctx, ev)
		}
	}()

	cleanup := func() {
		log.Println("[ebpf] detaching eBPF programs")
		rd.Close()
		tp.Close()
		coll.Close()
	}

	return cleanup, nil
}

func emitSpan(ctx context.Context, ev event) {
	ip := intToIP(ev.Daddr)
	port := ntohs(ev.Dport)

	_, span := tracer.Start(ctx, "tcp_connect",
		trace.WithAttributes(
			attribute.String("ebpf.event.type", "tcp_connect"),
			attribute.Int("process.pid", int(ev.PID)),
			attribute.Int("process.uid", int(ev.UID)),
			attribute.String("net.peer.ip", ip),
			attribute.Int("net.peer.port", int(port)),
			attribute.Int64("ebpf.timestamp_ns", int64(ev.TimestampNs)),
		),
	)
	span.End()

	log.Printf("[ebpf] tcp_connect pid=%d → %s:%d", ev.PID, ip, port)
}

func intToIP(addr uint32) string {
	return net.IPv4(
		byte(addr),
		byte(addr>>8),
		byte(addr>>16),
		byte(addr>>24),
	).String()
}

func ntohs(n uint16) uint16 {
	return (n >> 8) | (n << 8)
}
