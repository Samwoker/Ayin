import type {
	HealthResponse,
	ReadyResponse,
	StatusResponse,
	SystemOverview,
	LoginRequest,
	LoginResponse,
	TokenPair,
	MeResponse,
	ApiKey,
	CreateApiKeyRequest,
	CreateApiKeyResponse,
	Incident,
	CreateIncidentRequest,
	IncidentListResponse,
	IncidentDetailResponse,
	Run,
	RunStep,
	RunDetailResponse,
	Runbook,
	CreateRunbookRequest,
	WebhookIngestResponse,
	ToolListResponse,
	ToolExecutionResult,
	Remediation,
	ProposeRemediationRequest,
	ApproveRemediationRequest,
	RejectRemediationRequest,
	RollbackRemediationRequest,
	RemediationListResponse,
	RemediationDetailResponse,
	RemediationApprovalResponse,
	RemediationRollbackResponse
} from './types';

export class ApiClient {
	private baseUrl: string;
	private customFetch?: typeof fetch;

	constructor(baseUrl?: string, customFetch?: typeof fetch) {
		if (baseUrl) {
			this.baseUrl = baseUrl.replace(/\/+$/, '');
		} else if (typeof window !== 'undefined') {
			this.baseUrl = window.location.port === '8080' ? '' : 'http://localhost:8080';
		} else {
			this.baseUrl = process.env.PUBLIC_API_URL || 'http://localhost:8080';
		}
		this.customFetch = customFetch;
	}

	private get fetcher(): typeof fetch {
		if (this.customFetch) {
			return this.customFetch;
		}
		if (typeof window !== 'undefined') {
			return window.fetch.bind(window);
		}
		return fetch;
	}

	private async request<T>(path: string, options?: RequestInit, token?: string): Promise<T> {
		const url = `${this.baseUrl}${path.startsWith('/') ? path : `/${path}`}`;
		const controller = new AbortController();
		const timeoutId = setTimeout(() => controller.abort(), 6000);

		const headers: Record<string, string> = {
			Accept: 'application/json',
			...(options?.headers as Record<string, string>)
		};

		if (token) {
			if (token.startsWith('tr_live_')) {
				headers['X-API-Key'] = token;
			} else {
				headers['Authorization'] = `Bearer ${token}`;
			}
		}

		try {
			const res = await this.fetcher(url, {
				...options,
				signal: controller.signal,
				headers
			});

			clearTimeout(timeoutId);

			if (!res.ok) {
				const errorText = await res.text().catch(() => 'Unknown Error');
				throw new Error(`API Error [${res.status}]: ${errorText}`);
			}

			return (await res.json()) as T;
		} catch (err: unknown) {
			clearTimeout(timeoutId);
			if (err instanceof Error && err.name === 'AbortError') {
				throw new Error(`API request timed out: ${url}`);
			}
			throw err;
		}
	}

	// Probes & Status
	async getHealth(): Promise<HealthResponse> {
		return this.request<HealthResponse>('/healthz');
	}

	async getReady(): Promise<ReadyResponse> {
		return this.request<ReadyResponse>('/readyz');
	}

	async getStatus(): Promise<StatusResponse> {
		return this.request<StatusResponse>('/api/v1/status');
	}

	async checkSystemOverview(): Promise<SystemOverview> {
		const start = Date.now();
		try {
			const [health, ready, status] = await Promise.all([
				this.getHealth(),
				this.getReady(),
				this.getStatus()
			]);

			return {
				connected: true,
				latencyMs: Date.now() - start,
				health,
				ready,
				status,
				lastChecked: new Date().toISOString()
			};
		} catch (err: unknown) {
			return {
				connected: false,
				latencyMs: Date.now() - start,
				lastChecked: new Date().toISOString(),
				errorMessage: err instanceof Error ? err.message : String(err)
			};
		}
	}

	// Auth & Identity
	async login(req: LoginRequest): Promise<LoginResponse> {
		return this.request<LoginResponse>('/api/v1/auth/login', {
			method: 'POST',
			headers: { 'Content-Type': 'application/json' },
			body: JSON.stringify(req)
		});
	}

	async refreshToken(refreshToken: string): Promise<TokenPair> {
		return this.request<TokenPair>('/api/v1/auth/refresh', {
			method: 'POST',
			headers: { 'Content-Type': 'application/json' },
			body: JSON.stringify({ refresh_token: refreshToken })
		});
	}

	async getMe(token: string): Promise<MeResponse> {
		return this.request<MeResponse>('/api/v1/auth/me', undefined, token);
	}

	async listApiKeys(token: string): Promise<ApiKey[]> {
		return this.request<ApiKey[]>('/api/v1/auth/api-keys', undefined, token);
	}

	async createApiKey(token: string, req: CreateApiKeyRequest): Promise<CreateApiKeyResponse> {
		return this.request<CreateApiKeyResponse>(
			'/api/v1/auth/api-keys',
			{
				method: 'POST',
				headers: { 'Content-Type': 'application/json' },
				body: JSON.stringify(req)
			},
			token
		);
	}

	async revokeApiKey(token: string, id: string): Promise<{ id: string; revoked_at: string }> {
		return this.request<{ id: string; revoked_at: string }>(
			`/api/v1/auth/api-keys/${id}`,
			{ method: 'DELETE' },
			token
		);
	}

	// Incidents
	async listIncidents(token?: string, limit = 50, offset = 0): Promise<IncidentListResponse> {
		return this.request<IncidentListResponse>(
			`/api/v1/incidents?limit=${limit}&offset=${offset}`,
			undefined,
			token
		);
	}

	async getIncident(id: string, token?: string): Promise<IncidentDetailResponse> {
		return this.request<IncidentDetailResponse>(`/api/v1/incidents/${id}`, undefined, token);
	}

	async createIncident(req: CreateIncidentRequest, token?: string): Promise<Incident> {
		return this.request<Incident>(
			'/api/v1/incidents',
			{
				method: 'POST',
				headers: { 'Content-Type': 'application/json' },
				body: JSON.stringify(req)
			},
			token
		);
	}

	async updateIncidentStatus(arg1: string, arg2: string | { status: string }, arg3?: string): Promise<Incident> {
		let token = arg3;
		let id = arg1;
		let status = typeof arg2 === 'string' ? arg2 : arg2.status;
		if (typeof arg2 === 'string' && (arg1.startsWith('ey') || arg1.startsWith('tr_live_') || arg1.startsWith('mock-'))) {
			token = arg1;
			id = arg2;
			status = arg3 || '';
		}
		return this.request<Incident>(
			`/api/v1/incidents/${id}/status`,
			{
				method: 'PATCH',
				headers: { 'Content-Type': 'application/json' },
				body: JSON.stringify({ status })
			},
			token
		);
	}

	// Runs
	async listIncidentRuns(incidentId: string, token?: string): Promise<Run[]> {
		return this.request<Run[]>(`/api/v1/incidents/${incidentId}/runs`, undefined, token);
	}

	async createRun(arg1: string, arg2?: string): Promise<Run> {
		let incidentId = arg1;
		let token = arg2;
		if (arg1.startsWith('ey') || arg1.startsWith('tr_live_') || arg1.startsWith('mock-')) {
			token = arg1;
			incidentId = arg2 || '';
		}
		return this.request<Run>(
			`/api/v1/incidents/${incidentId}/runs`,
			{ method: 'POST' },
			token
		);
	}

	async getRun(id: string, token?: string): Promise<RunDetailResponse> {
		return this.request<RunDetailResponse>(`/api/v1/runs/${id}`, undefined, token);
	}

	async getRunSteps(id: string, token?: string): Promise<RunStep[]> {
		return this.request<RunStep[]>(`/api/v1/runs/${id}/steps`, undefined, token);
	}

	// Runbooks
	async listRunbooks(token?: string): Promise<Runbook[]> {
		return this.request<Runbook[]>('/api/v1/runbooks', undefined, token);
	}

	async getRunbook(token: string, id: string): Promise<Runbook> {
		return this.request<Runbook>(`/api/v1/runbooks/${id}`, undefined, token);
	}

	async createRunbook(token: string, req: CreateRunbookRequest): Promise<Runbook> {
		return this.request<Runbook>(
			'/api/v1/runbooks',
			{
				method: 'POST',
				headers: { 'Content-Type': 'application/json' },
				body: JSON.stringify(req)
			},
			token
		);
	}

	async updateRunbook(token: string, id: string, req: CreateRunbookRequest): Promise<Runbook> {
		return this.request<Runbook>(
			`/api/v1/runbooks/${id}`,
			{
				method: 'PUT',
				headers: { 'Content-Type': 'application/json' },
				body: JSON.stringify(req)
			},
			token
		);
	}

	async deleteRunbook(token: string, id: string): Promise<{ status: string }> {
		return this.request<{ status: string }>(
			`/api/v1/runbooks/${id}`,
			{ method: 'DELETE' },
			token
		);
	}

	/**
	 * Phase 4: Ingest alerts from monitoring provider
	 */
	async ingestWebhook(
		source: 'alertmanager' | 'grafana' | 'pagerduty' | 'generic',
		payload: unknown,
		options?: {
			orgId?: string;
			signature?: string;
			timestamp?: string;
			token?: string;
		}
	): Promise<WebhookIngestResponse> {
		const headers: Record<string, string> = {
			'Content-Type': 'application/json'
		};
		if (options?.signature) {
			headers['X-Signature-256'] = options.signature;
		}
		if (options?.timestamp) {
			headers['X-Timestamp'] = options.timestamp;
		}
		if (options?.orgId) {
			headers['X-Org-ID'] = options.orgId;
		}

		const path = options?.orgId
			? `/api/v1/ingest/${options.orgId}/webhooks/${source}`
			: `/api/v1/ingest/webhooks/${source}`;

		return this.request<WebhookIngestResponse>(
			path,
			{
				method: 'POST',
				headers,
				body: JSON.stringify(payload)
			},
			options?.token
		);
	}

	/**
	 * Phase 5: List investigation tools available to tenant
	 */
	async listTools(token?: string): Promise<ToolListResponse> {
		return this.request<ToolListResponse>('/api/v1/tools', undefined, token);
	}

	/**
	 * Phase 5: Execute an investigation tool on-demand
	 */
	async executeTool(name: string, params: Record<string, unknown>, token?: string): Promise<ToolExecutionResult> {
		return this.request<ToolExecutionResult>(
			`/api/v1/tools/${name}/execute`,
			{
				method: 'POST',
				headers: { 'Content-Type': 'application/json' },
				body: JSON.stringify(params)
			},
			token
		);
	}
	/**
	 * Phase 7: List remediations for an incident
	 */
	async listRemediationsForIncident(incidentId: string, token?: string): Promise<RemediationListResponse> {
		return this.request<RemediationListResponse>(`/api/v1/incidents/${incidentId}/remediations`, undefined, token);
	}

	/**
	 * Phase 7: Propose a remediation for an incident
	 */
	async proposeRemediation(incidentId: string, req: ProposeRemediationRequest, token?: string): Promise<Remediation> {
		return this.request<Remediation>(
			`/api/v1/incidents/${incidentId}/remediations`,
			{
				method: 'POST',
				headers: { 'Content-Type': 'application/json' },
				body: JSON.stringify(req)
			},
			token
		);
	}

	/**
	 * Phase 7: Get remediation details and audit event trail
	 */
	async getRemediation(remediationId: string, token?: string): Promise<RemediationDetailResponse> {
		return this.request<RemediationDetailResponse>(`/api/v1/remediations/${remediationId}`, undefined, token);
	}

	/**
	 * Phase 7: Approve and execute a proposed remediation
	 */
	async approveRemediation(remediationId: string, req?: ApproveRemediationRequest, token?: string): Promise<RemediationApprovalResponse> {
		return this.request<RemediationApprovalResponse>(
			`/api/v1/remediations/${remediationId}/approve`,
			{
				method: 'POST',
				headers: { 'Content-Type': 'application/json' },
				body: JSON.stringify(req || {})
			},
			token
		);
	}

	/**
	 * Phase 7: Reject a proposed remediation
	 */
	async rejectRemediation(remediationId: string, req?: RejectRemediationRequest, token?: string): Promise<Remediation> {
		return this.request<Remediation>(
			`/api/v1/remediations/${remediationId}/reject`,
			{
				method: 'POST',
				headers: { 'Content-Type': 'application/json' },
				body: JSON.stringify(req || {})
			},
			token
		);
	}

	/**
	 * Phase 7: Rollback a succeeded remediation
	 */
	async rollbackRemediation(remediationId: string, req?: RollbackRemediationRequest, token?: string): Promise<RemediationRollbackResponse> {
		return this.request<RemediationRollbackResponse>(
			`/api/v1/remediations/${remediationId}/rollback`,
			{
				method: 'POST',
				headers: { 'Content-Type': 'application/json' },
				body: JSON.stringify(req || {})
			},
			token
		);
	}
}

export const apiClient = new ApiClient();
