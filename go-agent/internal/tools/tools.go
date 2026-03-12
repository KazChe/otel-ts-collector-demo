package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	openai "github.com/sashabaranov/go-openai"
)

// Tool represents a callable tool available to the LLM agent.
type Tool struct {
	Name        string
	Description string
	Parameters  map[string]any
	Execute     func(ctx context.Context, args map[string]any) (string, error)
}

// Registry holds the set of tools the agent can invoke.
type Registry struct {
	tools map[string]Tool
}

// NewRegistry creates a Registry with the default tool set.
func NewRegistry() *Registry {
	r := &Registry{tools: make(map[string]Tool)}

	r.Register(Tool{
		Name:        "web_search",
		Description: "Search the web for information",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"query": map[string]any{
					"type":        "string",
					"description": "The search query",
				},
			},
			"required": []string{"query"},
		},
		Execute: func(ctx context.Context, args map[string]any) (string, error) {
			query, _ := args["query"].(string)
			return fmt.Sprintf(`Results for "%s":
1. HNSW (Hierarchical Navigable Small World) is a graph-based approximate nearest neighbor search algorithm created by Yury Malkov and Dmitry Yashunin (2016).
2. It builds a multi-layer graph where the top layers have long-range connections for fast traversal, and bottom layers have short-range connections for precision.
3. HNSW is widely used in vector databases like Pinecone, Weaviate, Qdrant, and pgvector for similarity search in high-dimensional embedding spaces.
4. Time complexity: O(log n) for search, making it significantly faster than brute-force kNN at scale.`, query), nil
		},
	})

	r.Register(Tool{
		Name:        "get_time",
		Description: "Get the current date and time",
		Parameters: map[string]any{
			"type":       "object",
			"properties": map[string]any{},
		},
		Execute: func(ctx context.Context, args map[string]any) (string, error) {
			return time.Now().Format(time.RFC3339), nil
		},
	})

	return r
}

// Register adds a tool to the registry.
func (r *Registry) Register(t Tool) {
	r.tools[t.Name] = t
}

// Get returns a tool by name.
func (r *Registry) Get(name string) (Tool, bool) {
	t, ok := r.tools[name]
	return t, ok
}

// List returns all registered tool names.
func (r *Registry) List() []string {
	names := make([]string, 0, len(r.tools))
	for name := range r.tools {
		names = append(names, name)
	}
	return names
}

// OpenAITools returns the tool definitions formatted for the OpenAI API.
func (r *Registry) OpenAITools() []openai.Tool {
	var oaiTools []openai.Tool
	for _, t := range r.tools {
		params, _ := json.Marshal(t.Parameters)
		oaiTools = append(oaiTools, openai.Tool{
			Type: openai.ToolTypeFunction,
			Function: &openai.FunctionDefinition{
				Name:        t.Name,
				Description: t.Description,
				Parameters:  json.RawMessage(params),
			},
		})
	}
	return oaiTools
}
