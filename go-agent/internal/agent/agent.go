package agent

import (
	"context"
	"fmt"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"

	"github.com/KazChe/otel-ts-collector-demo/go-agent/internal/tools"
)

var tracer = otel.Tracer("go-otel-agent")

// Agent holds the LLM client configuration.
type Agent struct {
	apiKey   string
	toolReg  *tools.Registry
}

// New creates an Agent with the given OpenAI API key.
func New(apiKey string) *Agent {
	return &Agent{
		apiKey:  apiKey,
		toolReg: tools.NewRegistry(),
	}
}

// Run executes a multi-turn conversation with tool use.
func (a *Agent) Run(ctx context.Context) error {
	ctx, span := tracer.Start(ctx, "invoke_agent",
		trace.WithAttributes(
			attribute.String("gen_ai.operation.name", "invoke_agent"),
			attribute.String("gen_ai.agent.name", "go_assistant"),
		),
	)
	defer span.End()

	// TODO: Implement real OpenAI chat completion calls
	// 1. Send user message to LLM
	// 2. If LLM returns tool_call, execute tool via a.toolReg
	// 3. Send tool result back to LLM for final answer
	// 4. Record GenAI span attributes (model, tokens, finish_reason)

	fmt.Println("[agent] TODO: implement LLM conversation loop")
	span.SetStatus(codes.Ok, "")

	return nil
}
