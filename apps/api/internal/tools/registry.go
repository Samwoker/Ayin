package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"runtime/debug"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
)

// ToolRegistry maintains available tools and handles dispatched execution within tenant constraints.
type ToolRegistry struct {
	mu             sync.RWMutex
	tools          map[string]Tool
	policy         PolicyProvider
	defaultTimeout time.Duration
}

// NewToolRegistry constructs a ToolRegistry.
func NewToolRegistry(policy PolicyProvider, defaultTimeout time.Duration) *ToolRegistry {
	if policy == nil {
		policy = NewAllowAllPolicyProvider()
	}
	if defaultTimeout <= 0 {
		defaultTimeout = 15 * time.Second
	}
	return &ToolRegistry{
		tools:          make(map[string]Tool),
		policy:         policy,
		defaultTimeout: defaultTimeout,
	}
}

// Register registers a tool implementation. Returns an error if a tool with the same name already exists.
func (r *ToolRegistry) Register(tool Tool) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if tool == nil {
		return fmt.Errorf("cannot register nil tool")
	}

	name := tool.Name()
	if name == "" {
		return fmt.Errorf("tool name cannot be empty")
	}

	if _, exists := r.tools[name]; exists {
		return fmt.Errorf("tool %q is already registered", name)
	}

	r.tools[name] = tool
	return nil
}

// Get returns the named tool if registered.
func (r *ToolRegistry) Get(name string) (Tool, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	t, ok := r.tools[name]
	return t, ok
}

// List returns all registered tools.
func (r *ToolRegistry) List() []Tool {
	r.mu.RLock()
	defer r.mu.RUnlock()

	out := make([]Tool, 0, len(r.tools))
	for _, t := range r.tools {
		out = append(out, t)
	}
	return out
}

// ListAvailable returns all tools that are permitted for the specified tenant organization.
func (r *ToolRegistry) ListAvailable(ctx context.Context, orgID pgtype.UUID) []Tool {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var available []Tool
	for name, t := range r.tools {
		if r.policy.IsToolAllowed(ctx, orgID, name) {
			available = append(available, t)
		}
	}
	return available
}

// Execute safely runs the named tool within tenant boundary, enforcing policy, timeout, and panic recovery.
func (r *ToolRegistry) Execute(ctx context.Context, orgID pgtype.UUID, name string, input json.RawMessage) ToolResult {
	start := time.Now()

	// 1. Tool existence check
	tool, exists := r.Get(name)
	if !exists {
		return ToolResult{
			Tool:      name,
			Success:   false,
			Error:     fmt.Sprintf("tool %q not found in registry", name),
			LatencyMs: time.Since(start).Milliseconds(),
		}
	}

	// 2. Tenancy & Policy allowlist check
	if !r.policy.IsToolAllowed(ctx, orgID, name) {
		return ToolResult{
			Tool:      name,
			Success:   false,
			Error:     fmt.Sprintf("tool %q is disabled or disallowed by organization policy", name),
			LatencyMs: time.Since(start).Milliseconds(),
		}
	}

	// 3. Execution context with per-call timeout
	execCtx, cancel := context.WithTimeout(ctx, r.defaultTimeout)
	defer cancel()

	var res ToolResult
	var execErr error

	// 4. Panic recovery wrapper
	func() {
		defer func() {
			if rec := recover(); rec != nil {
				execErr = fmt.Errorf("panic in tool %s: %v\nstack:\n%s", name, rec, string(debug.Stack()))
			}
		}()

		res, execErr = tool.Execute(execCtx, orgID, input)
	}()

	latencyMs := time.Since(start).Milliseconds()
	res.Tool = name
	res.LatencyMs = latencyMs

	if execErr != nil {
		res.Success = false
		res.Error = execErr.Error()
	}

	return res
}
