package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/KazChe/otel-ts-collector-demo/go-agent/internal/agent"
	"github.com/KazChe/otel-ts-collector-demo/go-agent/internal/ebpf"
	"github.com/KazChe/otel-ts-collector-demo/go-agent/internal/tracing"
)

func main() {
	enableEBPF := flag.Bool("ebpf", true, "enable eBPF observability layer")
	flag.Parse()

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	// Initialize OTEL tracing
	shutdown, err := tracing.Init(ctx)
	if err != nil {
		log.Fatalf("failed to init tracing: %v", err)
	}
	defer shutdown(ctx)

	// Start eBPF observability layer (optional, requires root)
	if *enableEBPF {
		cleanup, err := ebpf.Start(ctx)
		if err != nil {
			log.Printf("eBPF disabled: %v", err)
		} else {
			defer cleanup()
		}
	}

	// Run the LLM agent
	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		log.Fatal("OPENAI_API_KEY is required")
	}

	a := agent.New(apiKey)
	if err := a.Run(ctx); err != nil {
		log.Fatalf("agent error: %v", err)
	}

	fmt.Println("Done. Check Jaeger at http://localhost:16686 and Galileo for traces.")
}
