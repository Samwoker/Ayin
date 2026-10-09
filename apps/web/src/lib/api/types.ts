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

// Phase 4: Webhook Ingest Types
export interface WebhookIngestResponse {
	status: string;
	source: string;
	alerts_processed: number;
	incidents_created: number;
	incidents_grouped: number;
	incident_ids?: string[];
	message?: string;
}

// Phase 5: Investigation Tools Types
export interface ToolPropertyDoc {
	type: string;
	description?: string;
	enum?: string[];
	format?: string;
	default?: unknown;
}

export interface ToolSchema {
	type: string;
	properties: Record<string, ToolPropertyDoc>;
	required?: string[];
	description?: string;
}

export interface ToolInfo {
	name: string;
	description: string;
	schema: ToolSchema;
}

export interface ToolListResponse {
	tools: ToolInfo[];
}

export interface ToolExecutionResult {
	tool: string;
	success: boolean;
	data?: Record<string, unknown>;
	error?: string;
	latency_ms: number;
	metadata?: Record<string, unknown>;
}

// Phase 7: Remediations & Human-in-the-Loop (HITL)
export type RemediationActionType =
	| 'restart_service'
	| 'scale_service'
	| 'rollback_deploy'
	| 'clear_cache'
	| 'pause_consumer'
	| 'toggle_feature_flag';

export type RemediationRiskLevel = 'low' | 'medium' | 'high' | 'critical';

export type RemediationStatus =
	| 'proposed'
	| 'approved'
	| 'rejected'
	| 'expired'
	| 'executing'
	| 'succeeded'
	| 'failed'
	| 'rolled_back';

export interface Remediation {
	id: string;
	org_id: string;
	incident_id: string;
	hypothesis_id?: string | null;
	action_type: RemediationActionType;
	target: string;
	params: Record<string, unknown>;
	rationale: string;
	risk_level: RemediationRiskLevel;
	expected_effect: string;
	rollback_plan: string;
	status: RemediationStatus;
	expires_at: string;
	created_at: string;
	updated_at: string;
}

export interface RemediationEvent {
	id: string;
	org_id: string;
	remediation_id: string;
	transition_from: string;
	transition_to: string;
	user_id?: string | null;
	reason: string;
	created_at: string;
}

export interface ProposeRemediationRequest {
	action_type: RemediationActionType;
	target: string;
	params?: Record<string, unknown>;
	rationale: string;
	risk_level?: RemediationRiskLevel;
	expected_effect?: string;
	rollback_plan?: string;
	hypothesis_id?: string;
	ttl_seconds?: number;
}

export interface ApproveRemediationRequest {
	reason?: string;
}

export interface RejectRemediationRequest {
	reason?: string;
}

export interface RollbackRemediationRequest {
	reason?: string;
}

export interface RemediationListResponse {
	remediations: Remediation[];
	count: number;
}

export interface RemediationDetailResponse {
	remediation: Remediation;
	events: RemediationEvent[];
}

export interface RemediationApprovalResponse {
	remediation: Remediation;
	execution?: Record<string, unknown>;
}

export interface RemediationRollbackResponse {
	remediation: Remediation;
	rollback?: Record<string, unknown>;
}

