package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	"github.com/google/uuid"
	openai "github.com/sashabaranov/go-openai"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"

	"github.com/KazChe/otel-ts-collector-demo/go-agent/internal/tools"
)

var tracer = otel.Tracer("go-otel-agent")

const model = "gpt-4o-mini"

// Agent holds the LLM client configuration.
type Agent struct {
	client  *openai.Client
	toolReg *tools.Registry
}

// New creates an Agent with the given OpenAI API key.
func New(apiKey string) *Agent {
	return &Agent{
		client:  openai.NewClient(apiKey),
		toolReg: tools.NewRegistry(),
	}
}

// Run executes a multi-turn conversation with tool use.
func (a *Agent) Run(ctx context.Context) error {
	sessionID := uuid.New().String()

	ctx, rootSpan := tracer.Start(ctx, "invoke_agent",
		trace.WithAttributes(
			attribute.String("gen_ai.operation.name", "invoke_agent"),
			attribute.String("gen_ai.agent.name", "go_assistant"),
			attribute.String("session.id", sessionID),
		),
	)
	defer rootSpan.End()

	userMessage := "Explain what HNSW (Hierarchical Navigable Small World) is and how it's used in vector databases. Use web_search to find current information."

	messages := []openai.ChatCompletionMessage{
		{
			Role:    openai.ChatMessageRoleSystem,
			Content: "You are a helpful assistant that explains technical concepts clearly. Use the available tools when asked to search for information.",
		},
		{
			Role:    openai.ChatMessageRoleUser,
			Content: userMessage,
		},
	}

	// First LLM call
	resp, err := a.chat(ctx, messages)
	if err != nil {
		rootSpan.SetStatus(codes.Error, err.Error())
		return fmt.Errorf("chat completion: %w", err)
	}

	choice := resp.Choices[0]

	// Tool call loop
	for choice.FinishReason == openai.FinishReasonToolCalls {
		// Append the assistant message with tool calls
		messages = append(messages, choice.Message)

		for _, tc := range choice.Message.ToolCalls {
			result, execErr := a.executeTool(ctx, tc)
			if execErr != nil {
				result = fmt.Sprintf("error: %v", execErr)
			}

			messages = append(messages, openai.ChatCompletionMessage{
				Role:       openai.ChatMessageRoleTool,
				Content:    result,
				ToolCallID: tc.ID,
			})
		}

		// Follow-up LLM call with tool results
		resp, err = a.chat(ctx, messages)
		if err != nil {
			rootSpan.SetStatus(codes.Error, err.Error())
			return fmt.Errorf("chat completion (follow-up): %w", err)
		}
		choice = resp.Choices[0]
	}

	// Print final answer
	fmt.Println("\n--- Agent Response ---")
	fmt.Println(choice.Message.Content)
	fmt.Println("--- End ---")

	rootSpan.SetStatus(codes.Ok, "")
	return nil
}

// chat sends a chat completion request and records an OTEL span.
func (a *Agent) chat(ctx context.Context, messages []openai.ChatCompletionMessage) (openai.ChatCompletionResponse, error) {
	ctx, span := tracer.Start(ctx, "chat",
		trace.WithAttributes(
			attribute.String("gen_ai.operation.name", "chat"),
			attribute.String("gen_ai.request.model", model),
			attribute.String("gen_ai.provider.name", "openai"),
		),
	)
	defer span.End()

	// Record user message event
	if len(messages) > 0 {
		last := messages[len(messages)-1]
		span.AddEvent("gen_ai.user.message", trace.WithAttributes(
			attribute.String("role", last.Role),
			attribute.String("content", truncate(last.Content, 200)),
		))
	}

	resp, err := a.client.CreateChatCompletion(ctx, openai.ChatCompletionRequest{
		Model:    model,
		Messages: messages,
		Tools:    a.toolReg.OpenAITools(),
	})
	if err != nil {
		span.SetStatus(codes.Error, err.Error())
		return resp, err
	}

	// Record response attributes
	span.SetAttributes(
		attribute.Int("gen_ai.usage.input_tokens", resp.Usage.PromptTokens),
		attribute.Int("gen_ai.usage.output_tokens", resp.Usage.CompletionTokens),
		attribute.String("gen_ai.response.finish_reasons", string(resp.Choices[0].FinishReason)),
	)

	// Record choice event
	choiceContent := resp.Choices[0].Message.Content
	if choiceContent == "" && len(resp.Choices[0].Message.ToolCalls) > 0 {
		choiceContent = fmt.Sprintf("[tool_calls: %d]", len(resp.Choices[0].Message.ToolCalls))
	}
	span.AddEvent("gen_ai.choice", trace.WithAttributes(
		attribute.String("finish_reason", string(resp.Choices[0].FinishReason)),
		attribute.String("message", truncate(choiceContent, 200)),
	))

	span.SetStatus(codes.Ok, "")
	log.Printf("[chat] model=%s tokens_in=%d tokens_out=%d finish=%s",
		model, resp.Usage.PromptTokens, resp.Usage.CompletionTokens, resp.Choices[0].FinishReason)

	return resp, nil
}

// executeTool runs a tool call and records an OTEL span.
func (a *Agent) executeTool(ctx context.Context, tc openai.ToolCall) (string, error) {
	_, span := tracer.Start(ctx, "execute_tool",
		trace.WithAttributes(
			attribute.String("gen_ai.operation.name", "execute_tool"),
			attribute.String("gen_ai.tool.name", tc.Function.Name),
			attribute.String("gen_ai.tool.call.arguments", tc.Function.Arguments),
		),
	)
	defer span.End()

	tool, ok := a.toolReg.Get(tc.Function.Name)
	if !ok {
		err := fmt.Errorf("unknown tool: %s", tc.Function.Name)
		span.SetStatus(codes.Error, err.Error())
		return "", err
	}

	var args map[string]any
	if err := json.Unmarshal([]byte(tc.Function.Arguments), &args); err != nil {
		span.SetStatus(codes.Error, err.Error())
		return "", fmt.Errorf("parsing tool args: %w", err)
	}

	result, err := tool.Execute(ctx, args)
	if err != nil {
		span.SetStatus(codes.Error, err.Error())
		return "", err
	}

	span.SetAttributes(attribute.String("gen_ai.tool.call.result", truncate(result, 500)))
	span.SetStatus(codes.Ok, "")
	log.Printf("[tool] %s → %s", tc.Function.Name, truncate(result, 100))

	return result, nil
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "..."
}
