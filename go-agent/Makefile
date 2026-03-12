# Load root .env if it exists (-include ignores missing file)
-include ../.env
export

.PHONY: build run clean generate-ebpf

# eBPF generation requires clang and bpftool
generate-ebpf:
	clang -O2 -g -target bpf -I/usr/include/aarch64-linux-gnu -c bpf/trace.c -o bpf/trace.o

build: generate-ebpf
	go build -o bin/agent ./cmd/agent

run: build
	sudo OPENAI_API_KEY=$(OPENAI_API_KEY) OTEL_EXPORTER_OTLP_ENDPOINT=$(OTEL_EXPORTER_OTLP_ENDPOINT) ./bin/agent

# Run without eBPF (for development without root)
run-no-ebpf:
	go run ./cmd/agent -ebpf=false

clean:
	rm -rf bin/
	rm -f bpf/*.o

tidy:
	go mod tidy
