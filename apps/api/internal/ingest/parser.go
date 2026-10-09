package ingest

import (
	"strings"
)

// ParserRegistry manages alert parsers by source name.
type ParserRegistry struct {
	parsers map[string]Parser
}

// NewDefaultParserRegistry registers Alertmanager, Grafana, PagerDuty, and Generic parsers.
func NewDefaultParserRegistry() *ParserRegistry {
	r := &ParserRegistry{
		parsers: make(map[string]Parser),
	}
	r.Register(NewAlertmanagerParser())
	r.Register(NewGrafanaParser())
	r.Register(NewPagerDutyParser())
	r.Register(NewGenericParser())
	return r
}

// Register registers a new parser.
func (r *ParserRegistry) Register(p Parser) {
	r.parsers[strings.ToLower(p.Name())] = p
}

// Get retrieves a parser by source identifier.
func (r *ParserRegistry) Get(source string) (Parser, error) {
	clean := strings.ToLower(strings.TrimSpace(source))
	p, exists := r.parsers[clean]
	if !exists {
		return nil, ErrUnknownSource
	}
	return p, nil
}
