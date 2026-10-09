package tools

import (
	"context"
	"fmt"
	"sync"

	"github.com/jackc/pgx/v5/pgtype"
)

// PolicyProvider determines whether a specific tool is permitted to run for a given tenant organization.
type PolicyProvider interface {
	IsToolAllowed(ctx context.Context, orgID pgtype.UUID, toolName string) bool
}

// AllowAllPolicyProvider permits all tools without restriction.
type AllowAllPolicyProvider struct{}

func NewAllowAllPolicyProvider() *AllowAllPolicyProvider {
	return &AllowAllPolicyProvider{}
}

func (p *AllowAllPolicyProvider) IsToolAllowed(ctx context.Context, orgID pgtype.UUID, toolName string) bool {
	return true
}

// AllowlistPolicyProvider provides explicit per-organization tool allowlists and denylists.
type AllowlistPolicyProvider struct {
	mu           sync.RWMutex
	globalDenied map[string]bool
	orgAllowed   map[string]map[string]bool // orgUUIDString -> set of allowed tools
	orgDenied    map[string]map[string]bool // orgUUIDString -> set of explicitly denied tools
}

func NewAllowlistPolicyProvider() *AllowlistPolicyProvider {
	return &AllowlistPolicyProvider{
		globalDenied: make(map[string]bool),
		orgAllowed:   make(map[string]map[string]bool),
		orgDenied:    make(map[string]map[string]bool),
	}
}

func orgKey(orgID pgtype.UUID) string {
	return fmt.Sprintf("%x-%x-%x-%x-%x", orgID.Bytes[0:4], orgID.Bytes[4:6], orgID.Bytes[6:8], orgID.Bytes[8:10], orgID.Bytes[10:16])
}

// SetGlobalDeny blocks a tool globally across all organizations.
func (p *AllowlistPolicyProvider) SetGlobalDeny(toolName string, denied bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if denied {
		p.globalDenied[toolName] = true
	} else {
		delete(p.globalDenied, toolName)
	}
}

// SetOrgAllowed configures an explicit allowlist of tools for an organization.
func (p *AllowlistPolicyProvider) SetOrgAllowed(orgID pgtype.UUID, allowedTools []string) {
	p.mu.Lock()
	defer p.mu.Unlock()

	k := orgKey(orgID)
	set := make(map[string]bool)
	for _, t := range allowedTools {
		set[t] = true
	}
	p.orgAllowed[k] = set
}

// SetOrgToolDeny explicitly blocks an individual tool for a specific organization.
func (p *AllowlistPolicyProvider) SetOrgToolDeny(orgID pgtype.UUID, toolName string, denied bool) {
	p.mu.Lock()
	defer p.mu.Unlock()

	k := orgKey(orgID)
	if _, ok := p.orgDenied[k]; !ok {
		p.orgDenied[k] = make(map[string]bool)
	}
	if denied {
		p.orgDenied[k][toolName] = true
	} else {
		delete(p.orgDenied[k], toolName)
	}
}

func (p *AllowlistPolicyProvider) IsToolAllowed(ctx context.Context, orgID pgtype.UUID, toolName string) bool {
	p.mu.RLock()
	defer p.mu.RUnlock()

	// 1. Global denial takes highest precedence
	if p.globalDenied[toolName] {
		return false
	}

	k := orgKey(orgID)

	// 2. Organization-specific denial
	if deniedSet, ok := p.orgDenied[k]; ok && deniedSet[toolName] {
		return false
	}

	// 3. Organization-specific allowlist: if configured, tool must be present in set
	if allowedSet, ok := p.orgAllowed[k]; ok {
		return allowedSet[toolName]
	}

	// 4. By default, tool is allowed if no restrictive policy was set
	return true
}
