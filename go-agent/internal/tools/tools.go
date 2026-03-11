package tools

import (
	"context"
	"fmt"
)

// Tool represents a callable tool available to the LLM agent.
type Tool struct {
	Name        string
	Description string
	Execute     func(ctx context.Context, args map[string]any) (string, error)
}

// Registry holds the set of tools the agent can invoke.
type Registry struct {
	tools map[string]Tool
}

// NewRegistry creates a Registry with the default tool set.
func NewRegistry() *Registry {
	r := &Registry{tools: make(map[string]Tool)}

	// TODO: Register real tools
	r.Register(Tool{
		Name:        "web_search",
		Description: "Search the web for information",
		Execute: func(ctx context.Context, args map[string]any) (string, error) {
			query, _ := args["query"].(string)
			return fmt.Sprintf("search results for: %s", query), nil
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
