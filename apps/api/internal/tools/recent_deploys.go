package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
)

// Deployment records a release event for a service.
type Deployment struct {
	DeployID        string    `json:"deploy_id"`
	Service         string    `json:"service"`
	Environment     string    `json:"environment"`
	Version         string    `json:"version"`
	GitCommit       string    `json:"git_commit"`
	GitBranch       string    `json:"git_branch"`
	DeployedAt      time.Time `json:"deployed_at"`
	DeployedBy      string    `json:"deployed_by"`
	Status          string    `json:"status"` // succeeded, failed, rolled_back
	Changelog       string    `json:"changelog"`
	RollbackVersion string    `json:"rollback_version,omitempty"`
}

// DeployProvider fetches recent deployments for a service.
type DeployProvider interface {
	GetRecentDeploys(ctx context.Context, orgID pgtype.UUID, service string, env string, limit int, since time.Time) ([]Deployment, error)
}

// InMemoryDeployProvider provides in-memory deployment records for scenarios, demos, and testing.
type InMemoryDeployProvider struct {
	mu      sync.RWMutex
	deploys map[string][]Deployment // service -> deployments
}

func NewInMemoryDeployProvider() *InMemoryDeployProvider {
	now := time.Now().UTC()
	p := &InMemoryDeployProvider{
		deploys: make(map[string][]Deployment),
	}

	// Seed realistic deployment history for demo services
	p.AddDeploys("checkout-worker", []Deployment{
		{
			DeployID:        "dep-chk-091",
			Service:         "checkout-worker",
			Environment:     "production",
			Version:         "v2.14.0",
			GitCommit:       "7f2b1a9",
			GitBranch:       "main",
			DeployedAt:      now.Add(-22 * time.Minute),
			DeployedBy:      "sarah.c@acme.corp",
			Status:          "succeeded",
			Changelog:       "feat(queue): switch prefetch count from 10 to 1000 and enable aggressive batch ack",
			RollbackVersion: "v2.13.9",
		},
		{
			DeployID:        "dep-chk-090",
			Service:         "checkout-worker",
			Environment:     "production",
			Version:         "v2.13.9",
			GitCommit:       "3c8d0e1",
			GitBranch:       "main",
			DeployedAt:      now.Add(-48 * time.Hour),
			DeployedBy:      "alex.m@acme.corp",
			Status:          "succeeded",
			Changelog:       "fix(metrics): add prometheus counter for message processing latency",
			RollbackVersion: "v2.13.8",
		},
	})

	p.AddDeploys("payment-api", []Deployment{
		{
			DeployID:        "dep-pay-104",
			Service:         "payment-api",
			Environment:     "production",
			Version:         "v3.8.1",
			GitCommit:       "e410bc3",
			GitBranch:       "main",
			DeployedAt:      now.Add(-14 * time.Minute),
			DeployedBy:      "ci-runner@acme.corp",
			Status:          "succeeded",
			Changelog:       "fix(connection-pool): reduce max idle conns timeout from 10m to 30s",
			RollbackVersion: "v3.8.0",
		},
		{
			DeployID:        "dep-pay-103",
			Service:         "payment-api",
			Environment:     "production",
			Version:         "v3.8.0",
			GitCommit:       "a920df5",
			GitBranch:       "main",
			DeployedAt:      now.Add(-3 * 24 * time.Hour),
			DeployedBy:      "david.l@acme.corp",
			Status:          "succeeded",
			Changelog:       "feat(stripe): upgrade stripe-go library to v78.0",
			RollbackVersion: "v3.7.9",
		},
	})

	return p
}

func (p *InMemoryDeployProvider) AddDeploys(service string, items []Deployment) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.deploys[strings.ToLower(service)] = append(p.deploys[strings.ToLower(service)], items...)
}

func (p *InMemoryDeployProvider) GetRecentDeploys(ctx context.Context, orgID pgtype.UUID, service string, env string, limit int, since time.Time) ([]Deployment, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	svcKey := strings.ToLower(service)
	items, ok := p.deploys[svcKey]
	if !ok {
		return []Deployment{}, nil
	}

	var filtered []Deployment
	for _, d := range items {
		if env != "" && !strings.EqualFold(d.Environment, env) {
			continue
		}
		if !since.IsZero() && d.DeployedAt.Before(since) {
			continue
		}
		filtered = append(filtered, d)
		if limit > 0 && len(filtered) >= limit {
			break
		}
	}

	return filtered, nil
}

// RecentDeploysTool inspects recent deployments and commit changes for a service.
type RecentDeploysTool struct {
	provider DeployProvider
}

func NewRecentDeploysTool(provider DeployProvider) *RecentDeploysTool {
	if provider == nil {
		provider = NewInMemoryDeployProvider()
	}
	return &RecentDeploysTool{
		provider: provider,
	}
}

func (t *RecentDeploysTool) Name() string {
	return "recent_deploys"
}

func (t *RecentDeploysTool) Description() string {
	return "Fetches recent deployment events, git commits, deployers, and changelogs for a target service to identify regression culprits or faulty releases."
}

func (t *RecentDeploysTool) Schema() ToolSchema {
	return ToolSchema{
		Type: "object",
		Required: []string{"service"},
		Properties: map[string]PropertyDoc{
			"service": {
				Type:        "string",
				Description: "Service name to inspect (e.g. 'checkout-worker', 'payment-api').",
			},
			"environment": {
				Type:        "string",
				Description: "Environment filter (e.g. 'production', 'staging'). Defaults to all environments.",
			},
			"limit": {
				Type:        "integer",
				Description: "Maximum number of deployments to return (default: 5).",
				Default:     5,
			},
			"since": {
				Type:        "string",
				Format:      "date-time",
				Description: "Optional RFC3339 timestamp to retrieve deploys occurring after this point.",
			},
		},
	}
}

type recentDeploysInput struct {
	Service     string `json:"service"`
	Environment string `json:"environment"`
	Limit       int    `json:"limit"`
	Since       string `json:"since"`
}

func (t *RecentDeploysTool) Execute(ctx context.Context, orgID pgtype.UUID, rawInput json.RawMessage) (ToolResult, error) {
	var in recentDeploysInput
	if len(rawInput) > 0 {
		if err := json.Unmarshal(rawInput, &in); err != nil {
			return ToolResult{
				Success: false,
				Error:   fmt.Sprintf("invalid input parameters: %v", err),
			}, nil
		}
	}

	if in.Service == "" {
		return ToolResult{
			Success: false,
			Error:   "parameter 'service' is required",
		}, nil
	}

	limit := in.Limit
	if limit <= 0 {
		limit = 5
	}

	var sinceTime time.Time
	if in.Since != "" {
		if parsed, err := time.Parse(time.RFC3339, in.Since); err == nil {
			sinceTime = parsed
		}
	}

	deploys, err := t.provider.GetRecentDeploys(ctx, orgID, in.Service, in.Environment, limit, sinceTime)
	if err != nil {
		return ToolResult{
			Success: false,
			Error:   fmt.Sprintf("failed to retrieve deployments: %v", err),
		}, nil
	}

	return ToolResult{
		Success: true,
		Data: map[string]any{
			"service":          in.Service,
			"environment":      in.Environment,
			"total_found":      len(deploys),
			"recent_deploys":   deploys,
		},
		Metadata: map[string]any{
			"count": len(deploys),
		},
	}, nil
}
