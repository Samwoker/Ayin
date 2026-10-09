/**
 * Types generated from and conforming to api/openapi.yaml
 */

export interface HealthResponse {
	status: string;
	timestamp: string;
	service: string;
}

export interface ReadyResponse {
	status: string;
	timestamp: string;
	checks: Record<string, string>;
}

export interface StatusResponse {
	status: string;
	version: string;
	service: string;
	environment: string;
	uptime_seconds: number;
	timestamp: string;
	components: Record<string, string>;
}

export interface ApiError {
	error: string;
	request_id?: string;
	status?: number;
}

export interface SystemOverview {
	connected: boolean;
	latencyMs: number;
	health?: HealthResponse;
	ready?: ReadyResponse;
	status?: StatusResponse;
	lastChecked: string;
	errorMessage?: string;
}

// Phase 3: Auth & Identity Types
export interface LoginRequest {
	email: string;
	password: string;
	org_id?: string;
}

export interface UserDTO {
	id: string;
	email: string;
	name: string;
	role: string;
}

export interface OrgDTO {
	id: string;
	name: string;
	slug: string;
}

export interface LoginResponse {
	access_token: string;
	refresh_token: string;
	expires_in: number;
	token_type: string;
	user: UserDTO;
	org: OrgDTO;
}

export interface RefreshRequest {
	refresh_token: string;
}

export interface TokenPair {
	access_token: string;
	refresh_token: string;
	expires_in: number;
	token_type: string;
}

export interface MeResponse {
	user_id: string;
	org_id: string;
	role: string;
	email: string;
	is_apikey: boolean;
}

export interface ApiKey {
	id: string;
	org_id: string;
	name: string;
	key_prefix: string;
	created_at: string;
	expires_at?: string;
	revoked_at?: string;
}

export interface CreateApiKeyRequest {
	name: string;
	expires_in?: string;
}

export interface CreateApiKeyResponse {
	id: string;
	name: string;
	key_prefix: string;
	raw_key: string;
	created_at: string;
}

// Incidents
export interface Incident {
	id: string;
	org_id: string;
	title: string;
	service: string;
	severity: 'sev1' | 'sev2' | 'sev3' | 'sev4' | 'sev5' | string;
	status: 'open' | 'investigating' | 'mitigated' | 'resolved' | 'closed' | string;
	opened_at: string;
	resolved_at?: string;
	updated_at?: string;
}

export interface CreateIncidentRequest {
	title: string;
	service: string;
	severity?: string;
	status?: string;
}

export interface IncidentListResponse {
	incidents: Incident[];
	limit: number;
	offset: number;
}

export interface IncidentDetailResponse {
	incident: Incident;
	alerts: Record<string, unknown>[];
	events: Record<string, unknown>[];
}

// Runs & Steps
export interface Run {
	id: string;
	org_id: string;
	incident_id: string;
	prompt_version_id?: string;
	status: string;
	started_at: string;
	completed_at?: string;
	total_tokens?: number;
	total_cost_usd?: number;
}

export interface RunStep {
	id: string;
	run_id: string;
	step_number: number;
	step_type: string;
	tool_name?: string;
	tool_input?: Record<string, unknown>;
	tool_output?: Record<string, unknown>;
	tokens_in?: number;
	tokens_out?: number;
	latency_ms?: number;
	created_at: string;
}

export interface RunDetailResponse {
	run: Run;
	hypotheses: Record<string, unknown>[];
	steps: RunStep[];
}

// Runbooks
export interface Runbook {
	id: string;
	org_id: string;
	title: string;
	description: string;
	content_markdown: string;
	created_at: string;
	updated_at: string;
}

export interface CreateRunbookRequest {
	title: string;
	description?: string;
	content_markdown: string;
}
