package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"

	"github.com/jackc/pgx/v5/pgtype"
)

// ServiceNode represents architecture details and relationships for a microservice.
type ServiceNode struct {
	Service    string   `json:"service"`
	Tier       string   `json:"tier"` // tier-1, tier-2, tier-3
	Team       string   `json:"team"`
	Repo       string   `json:"repo"`
	Upstream   []string `json:"upstream"`   // Services that call this service
	Downstream []string `json:"downstream"` // APIs/Services called by this service
	Databases  []string `json:"databases"`  // Databases directly accessed
	Queues     []string `json:"queues"`     // Message queues consumed or published
	SLOTarget  string   `json:"slo_target"` // e.g. 99.95% availability, <250ms p95
}

// ServiceTopology describes a target service and its extended dependency graph.
type ServiceTopology struct {
	TargetService string                 `json:"target_service"`
	Primary       ServiceNode            `json:"primary"`
	Related       map[string]ServiceNode `json:"related,omitempty"`
}

// TopologyProvider fetches dependency topology information for a service.
type TopologyProvider interface {
	GetTopology(ctx context.Context, orgID pgtype.UUID, service string, depth int) (*ServiceTopology, error)
}

// InMemoryTopologyProvider stores service architecture mappings in memory.
type InMemoryTopologyProvider struct {
	mu    sync.RWMutex
	nodes map[string]ServiceNode
}

func NewInMemoryTopologyProvider() *InMemoryTopologyProvider {
	p := &InMemoryTopologyProvider{
		nodes: make(map[string]ServiceNode),
	}

	// Seed architecture for standard services
	p.AddNode(ServiceNode{
		Service:    "checkout-worker",
		Tier:       "tier-1",
		Team:       "Order Fulfillment",
		Repo:       "github.com/acme/checkout-worker",
		Upstream:   []string{"order-service"},
		Downstream: []string{"payment-api", "inventory-api"},
		Databases:  []string{"postgres-primary"},
		Queues:     []string{"rabbitmq:orders-exchange", "rabbitmq:checkout-events"},
		SLOTarget:  "99.9% uptime, <15s processing delay",
	})

	p.AddNode(ServiceNode{
		Service:    "payment-api",
		Tier:       "tier-1",
		Team:       "Payments & Billing",
		Repo:       "github.com/acme/payment-api",
		Upstream:   []string{"checkout-worker", "mobile-gateway", "web-gateway"},
		Downstream: []string{"stripe-external-api", "fraud-detection-service"},
		Databases:  []string{"postgres-primary", "redis-session-store"},
		Queues:     []string{"rabbitmq:payment-audit"},
		SLOTarget:  "99.99% availability, <150ms p95",
	})

	p.AddNode(ServiceNode{
		Service:    "order-service",
		Tier:       "tier-1",
		Team:       "E-Commerce Core",
		Repo:       "github.com/acme/order-service",
		Upstream:   []string{"web-gateway", "mobile-gateway"},
		Downstream: []string{"checkout-worker", "inventory-api"},
		Databases:  []string{"postgres-orders"},
		Queues:     []string{"rabbitmq:orders-exchange"},
		SLOTarget:  "99.9% availability, <200ms p95",
	})

	p.AddNode(ServiceNode{
		Service:    "inventory-api",
		Tier:       "tier-2",
		Team:       "Warehouse & Logistics",
		Repo:       "github.com/acme/inventory-api",
		Upstream:   []string{"order-service", "checkout-worker"},
		Downstream: []string{"warehouse-wms-sync"},
		Databases:  []string{"postgres-inventory"},
		Queues:     []string{},
		SLOTarget:  "99.5% availability, <300ms p95",
	})

	return p
}

func (p *InMemoryTopologyProvider) AddNode(node ServiceNode) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.nodes[strings.ToLower(node.Service)] = node
}

func (p *InMemoryTopologyProvider) GetTopology(ctx context.Context, orgID pgtype.UUID, service string, depth int) (*ServiceTopology, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	key := strings.ToLower(service)
	primary, ok := p.nodes[key]
	if !ok {
		// Provide a generic fallback node if service not explicitly registered
		primary = ServiceNode{
			Service:    service,
			Tier:       "tier-2",
			Team:       "Infrastructure",
			Upstream:   []string{},
			Downstream: []string{},
			Databases:  []string{"postgres"},
			Queues:     []string{},
			SLOTarget:  "99.9%",
		}
	}

	topo := &ServiceTopology{
		TargetService: service,
		Primary:       primary,
		Related:       make(map[string]ServiceNode),
	}

	if depth > 1 {
		// Populate related downstream nodes
		for _, down := range primary.Downstream {
			if relNode, found := p.nodes[strings.ToLower(down)]; found {
				topo.Related[down] = relNode
			}
		}
		// Populate related upstream nodes
		for _, up := range primary.Upstream {
			if relNode, found := p.nodes[strings.ToLower(up)]; found {
				topo.Related[up] = relNode
			}
		}
	}

	return topo, nil
}

// ServiceTopologyTool provides dependency map and upstream/downstream connection context.
type ServiceTopologyTool struct {
	provider TopologyProvider
}

func NewServiceTopologyTool(provider TopologyProvider) *ServiceTopologyTool {
	if provider == nil {
		provider = NewInMemoryTopologyProvider()
	}
	return &ServiceTopologyTool{
		provider: provider,
	}
}

func (t *ServiceTopologyTool) Name() string {
	return "service_topology"
}

func (t *ServiceTopologyTool) Description() string {
	return "Inspects the dependency graph for a service, including upstream consumers, downstream APIs, databases, message queues, and critical SLO tiers."
}

func (t *ServiceTopologyTool) Schema() ToolSchema {
	return ToolSchema{
		Type: "object",
		Required: []string{"service"},
		Properties: map[string]PropertyDoc{
			"service": {
				Type:        "string",
				Description: "The name of the service to inspect (e.g. 'checkout-worker', 'payment-api').",
			},
			"depth": {
				Type:        "integer",
				Description: "Depth of dependency traversal (1 = direct connections, 2 = include adjacent services). Defaults to 1.",
				Default:     1,
			},
		},
	}
}

type topologyInput struct {
	Service string `json:"service"`
	Depth   int    `json:"depth"`
}

func (t *ServiceTopologyTool) Execute(ctx context.Context, orgID pgtype.UUID, rawInput json.RawMessage) (ToolResult, error) {
	var in topologyInput
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

	depth := in.Depth
	if depth <= 0 {
		depth = 1
	}
	if depth > 3 {
		depth = 3
	}

	topo, err := t.provider.GetTopology(ctx, orgID, in.Service, depth)
	if err != nil {
		return ToolResult{
			Success: false,
			Error:   fmt.Sprintf("failed to get service topology: %v", err),
		}, nil
	}

	return ToolResult{
		Success: true,
		Data:    topo,
		Metadata: map[string]any{
			"service":          topo.TargetService,
			"tier":             topo.Primary.Tier,
			"team":             topo.Primary.Team,
			"downstream_count": len(topo.Primary.Downstream),
			"upstream_count":   len(topo.Primary.Upstream),
		},
	}, nil
}
