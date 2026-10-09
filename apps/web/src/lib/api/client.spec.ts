import { describe, it, expect, vi } from 'vitest';
import { ApiClient } from './client';

describe('ApiClient', () => {
	it('fetches healthz successfully', async () => {
		const mockResponse = {
			status: 'ok',
			timestamp: '2026-10-09T07:00:00Z',
			service: 'triage-api'
		};

		const mockFetch = vi.fn().mockResolvedValue({
			ok: true,
			json: async () => mockResponse
		} as Response);

		const client = new ApiClient('http://test-host:8080', mockFetch);
		const result = await client.getHealth();

		expect(mockFetch).toHaveBeenCalledWith(
			'http://test-host:8080/healthz',
			expect.objectContaining({
				headers: expect.objectContaining({ Accept: 'application/json' })
			})
		);
		expect(result).toEqual(mockResponse);
	});

	it('fetches status successfully', async () => {
		const mockResponse = {
			status: 'online',
			version: '0.1.0',
			service: 'triage-api',
			environment: 'development',
			uptime_seconds: 42,
			timestamp: '2026-10-09T07:00:00Z',
			components: { database: 'configured', redis: 'configured' }
		};

		const mockFetch = vi.fn().mockResolvedValue({
			ok: true,
			json: async () => mockResponse
		} as Response);

		const client = new ApiClient('http://test-host:8080', mockFetch);
		const result = await client.getStatus();

		expect(result.service).toBe('triage-api');
		expect(result.status).toBe('online');
		expect(result.version).toBe('0.1.0');
	});

	it('computes system overview and handles errors gracefully', async () => {
		const mockFetch = vi.fn().mockRejectedValue(new Error('Connection refused'));

		const client = new ApiClient('http://offline-host:8080', mockFetch);
		const overview = await client.checkSystemOverview();

		expect(overview.connected).toBe(false);
		expect(overview.errorMessage).toContain('Connection refused');
		expect(typeof overview.latencyMs).toBe('number');
	});

	it('authenticates user login and issues tokens', async () => {
		const mockLoginResponse = {
			access_token: 'jwt-access-token-123',
			refresh_token: 'jwt-refresh-token-456',
			expires_in: 900,
			token_type: 'Bearer',
			user: {
				id: 'u-1',
				email: 'alice@acme.com',
				name: 'Alice',
				role: 'owner'
			},
			org: {
				id: 'org-1',
				name: 'Acme Corp',
				slug: 'acme'
			}
		};

		const mockFetch = vi.fn().mockResolvedValue({
			ok: true,
			json: async () => mockLoginResponse
		} as Response);

		const client = new ApiClient('http://test-host:8080', mockFetch);
		const result = await client.login({
			email: 'alice@acme.com',
			password: 'password123'
		});

		expect(mockFetch).toHaveBeenCalledWith(
			'http://test-host:8080/api/v1/auth/login',
			expect.objectContaining({
				method: 'POST',
				headers: expect.objectContaining({
					'Content-Type': 'application/json'
				}),
				body: JSON.stringify({ email: 'alice@acme.com', password: 'password123' })
			})
		);
		expect(result.access_token).toBe('jwt-access-token-123');
		expect(result.user.role).toBe('owner');
	});

	it('passes Bearer token and fetches incidents', async () => {
		const mockIncidentsResponse = {
			incidents: [
				{
					id: 'inc-1',
					org_id: 'org-1',
					title: 'High CPU usage',
					service: 'api-service',
					severity: 'sev2',
					status: 'open',
					opened_at: '2026-10-09T08:00:00Z'
				}
			],
			limit: 50,
			offset: 0
		};

		const mockFetch = vi.fn().mockResolvedValue({
			ok: true,
			json: async () => mockIncidentsResponse
		} as Response);

		const client = new ApiClient('http://test-host:8080', mockFetch);
		const result = await client.listIncidents('mock-jwt-token');

		expect(mockFetch).toHaveBeenCalledWith(
			'http://test-host:8080/api/v1/incidents?limit=50&offset=0',
			expect.objectContaining({
				headers: expect.objectContaining({
					Authorization: 'Bearer mock-jwt-token'
				})
			})
		);
		expect(result.incidents).toHaveLength(1);
		expect(result.incidents[0].title).toBe('High CPU usage');
	});

	it('passes API Key header for tr_live_ keys', async () => {
		const mockRunbooks = [
			{
				id: 'rb-1',
				org_id: 'org-1',
				title: 'Postgres Failover',
				description: 'Steps for failover',
				content_markdown: '# Runbook',
				created_at: '2026-10-09T08:00:00Z',
				updated_at: '2026-10-09T08:00:00Z'
			}
		];

		const mockFetch = vi.fn().mockResolvedValue({
			ok: true,
			json: async () => mockRunbooks
		} as Response);

		const client = new ApiClient('http://test-host:8080', mockFetch);
		const result = await client.listRunbooks('tr_live_acme_randomkey123');

		expect(mockFetch).toHaveBeenCalledWith(
			'http://test-host:8080/api/v1/runbooks',
			expect.objectContaining({
				headers: expect.objectContaining({
					'X-API-Key': 'tr_live_acme_randomkey123'
				})
			})
		);
		expect(result).toHaveLength(1);
	});

	it('posts webhook payload with HMAC signature and metadata headers', async () => {
		const mockResponse = {
			status: 'success',
			source: 'alertmanager',
			alerts_processed: 2,
			incidents_created: 1,
			incidents_grouped: 1,
			incident_ids: ['inc-123']
		};

		const mockFetch = vi.fn().mockResolvedValue({
			ok: true,
			json: async () => mockResponse
		} as Response);

		const client = new ApiClient('http://test-host:8080', mockFetch);
		const payload = { alerts: [{ status: 'firing' }] };
		const result = await client.ingestWebhook('alertmanager', payload, {
			orgId: '00000000-0000-0000-0000-000000000001',
			signature: 'sha256=test_sig',
			timestamp: '1700000000'
		});

		expect(mockFetch).toHaveBeenCalledWith(
			'http://test-host:8080/api/v1/ingest/00000000-0000-0000-0000-000000000001/webhooks/alertmanager',
			expect.objectContaining({
				method: 'POST',
				headers: expect.objectContaining({
					'Content-Type': 'application/json',
					'X-Signature-256': 'sha256=test_sig',
					'X-Timestamp': '1700000000',
					'X-Org-ID': '00000000-0000-0000-0000-000000000001'
				}),
				body: JSON.stringify(payload)
			})
		);
		expect(result.status).toBe('success');
		expect(result.incidents_created).toBe(1);
	});

	it('fetches list of available investigation tools', async () => {
		const mockTools = {
			tools: [
				{
					name: 'log_search',
					description: 'Search logs',
					schema: { type: 'object', properties: {} }
				},
				{
					name: 'recent_deploys',
					description: 'Recent deploys',
					schema: { type: 'object', properties: {} }
				}
			]
		};

		const mockFetch = vi.fn().mockResolvedValue({
			ok: true,
			json: async () => mockTools
		} as Response);

		const client = new ApiClient('http://test-host:8080', mockFetch);
		const result = await client.listTools('jwt_token_123');

		expect(mockFetch).toHaveBeenCalledWith(
			'http://test-host:8080/api/v1/tools',
			expect.objectContaining({
				headers: expect.objectContaining({
					Authorization: 'Bearer jwt_token_123'
				})
			})
		);
		expect(result.tools).toHaveLength(2);
		expect(result.tools[0].name).toBe('log_search');
	});

	it('executes tool on-demand and returns structured result', async () => {
		const mockResult = {
			tool: 'service_topology',
			success: true,
			data: { target_service: 'checkout-worker', primary: { tier: 'tier-1' } },
			latency_ms: 12
		};

		const mockFetch = vi.fn().mockResolvedValue({
			ok: true,
			json: async () => mockResult
		} as Response);

		const client = new ApiClient('http://test-host:8080', mockFetch);
		const result = await client.executeTool(
			'service_topology',
			{ service: 'checkout-worker', depth: 1 },
			'jwt_token_123'
		);

		expect(mockFetch).toHaveBeenCalledWith(
			'http://test-host:8080/api/v1/tools/service_topology/execute',
			expect.objectContaining({
				method: 'POST',
				headers: expect.objectContaining({
					'Content-Type': 'application/json',
					Authorization: 'Bearer jwt_token_123'
				}),
				body: JSON.stringify({ service: 'checkout-worker', depth: 1 })
			})
		);
		expect(result.success).toBe(true);
		expect(result.tool).toBe('service_topology');
	});

	it('lists remediations for an incident', async () => {
		const mockResponse = {
			remediations: [
				{
					id: 'rem-1',
					action_type: 'restart_service',
					target: 'checkout-worker',
					status: 'proposed'
				}
			],
			count: 1
		};

		const mockFetch = vi.fn().mockResolvedValue({
			ok: true,
			json: async () => mockResponse
		} as Response);

		const client = new ApiClient('http://test-host:8080', mockFetch);
		const result = await client.listRemediationsForIncident('inc-123', 'jwt_token_123');

		expect(mockFetch).toHaveBeenCalledWith(
			'http://test-host:8080/api/v1/incidents/inc-123/remediations',
			expect.objectContaining({
				headers: expect.objectContaining({
					Authorization: 'Bearer jwt_token_123'
				})
			})
		);
		expect(result.remediations).toHaveLength(1);
		expect(result.remediations[0].action_type).toBe('restart_service');
	});

	it('proposes and approves remediation successfully', async () => {
		const mockProposed = {
			id: 'rem-1',
			incident_id: 'inc-123',
			action_type: 'scale_service',
			target: 'payment-api',
			status: 'proposed'
		};

		const mockApproved = {
			remediation: {
				id: 'rem-1',
				status: 'succeeded'
			},
			execution: { success: true }
		};

		const mockFetch = vi
			.fn()
			.mockResolvedValueOnce({
				ok: true,
				json: async () => mockProposed
			} as Response)
			.mockResolvedValueOnce({
				ok: true,
				json: async () => mockApproved
			} as Response);

		const client = new ApiClient('http://test-host:8080', mockFetch);
		const prop = await client.proposeRemediation(
			'inc-123',
			{
				action_type: 'scale_service',
				target: 'payment-api',
				rationale: 'Traffic spike'
			},
			'jwt_token_123'
		);
		expect(prop.id).toBe('rem-1');

		const app = await client.approveRemediation('rem-1', { reason: 'Looks good' }, 'jwt_token_123');
		expect(app.remediation.status).toBe('succeeded');
	});

	it('rejects and rolls back remediation', async () => {
		const mockRejected = {
			id: 'rem-2',
			status: 'rejected'
		};

		const mockRollback = {
			remediation: {
				id: 'rem-1',
				status: 'rolled_back'
			},
			rollback: { success: true }
		};

		const mockFetch = vi
			.fn()
			.mockResolvedValueOnce({
				ok: true,
				json: async () => mockRejected
			} as Response)
			.mockResolvedValueOnce({
				ok: true,
				json: async () => mockRollback
			} as Response);

		const client = new ApiClient('http://test-host:8080', mockFetch);
		const rej = await client.rejectRemediation('rem-2', { reason: 'Unsafe' }, 'jwt_token_123');
		expect(rej.status).toBe('rejected');

		const rb = await client.rollbackRemediation('rem-1', { reason: 'Revert test' }, 'jwt_token_123');
		expect(rb.remediation.status).toBe('rolled_back');
	});

	it('fetches billing plans, subscription, and initiates checkout and portal sessions', async () => {
		const mockPlans = {
			plans: [
				{ tier: 'free', name: 'Free', price_usd: 0, monthly_runs_limit: 10, rate_limit_rpm: 10, concurrency: 1, features: [] },
				{ tier: 'pro', name: 'Pro', price_usd: 49, monthly_runs_limit: 250, rate_limit_rpm: 60, concurrency: 5, features: [] }
			]
		};

		const mockSub = {
			org_id: 'org-123',
			plan_tier: 'pro',
			status: 'active',
			monthly_runs_limit: 250,
			monthly_runs_used: 12,
			monthly_runs_remaining: 238,
			reset_at: '2026-11-01T00:00:00Z'
		};

		const mockCheckout = { checkout_url: 'https://checkout.stripe.com/pay/cs_123' };
		const mockPortal = { portal_url: 'https://billing.stripe.com/p/session/bps_123' };

		const mockFetch = vi
			.fn()
			.mockResolvedValueOnce({
				ok: true,
				json: async () => mockPlans
			} as Response)
			.mockResolvedValueOnce({
				ok: true,
				json: async () => mockSub
			} as Response)
			.mockResolvedValueOnce({
				ok: true,
				json: async () => mockCheckout
			} as Response)
			.mockResolvedValueOnce({
				ok: true,
				json: async () => mockPortal
			} as Response);

		const client = new ApiClient('http://test-host:8080', mockFetch);

		const plansRes = await client.getBillingPlans();
		expect(plansRes.plans).toHaveLength(2);

		const subRes = await client.getSubscription('token_abc');
		expect(subRes.plan_tier).toBe('pro');
		expect(subRes.monthly_runs_remaining).toBe(238);

		const checkoutRes = await client.createCheckoutSession({ success_url: 'http://localhost/success' }, 'token_abc');
		expect(checkoutRes.checkout_url).toBe('https://checkout.stripe.com/pay/cs_123');

		const portalRes = await client.createPortalSession({ return_url: 'http://localhost/return' }, 'token_abc');
		expect(portalRes.portal_url).toBe('https://billing.stripe.com/p/session/bps_123');
	});
});



