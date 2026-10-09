import type {
	Incident,
	Run,
	RunStep,
	Remediation,
	RemediationEvent,
	Runbook
} from './api/types';

export interface HypothesisData {
	id: string;
	rank: number;
	root_cause: string;
	confidence: number;
	evidence: string[];
	checklist: string[];
}

export const DEMO_INCIDENTS: Incident[] = [
	{
		id: '40000000-0000-0000-0000-000000000001',
		org_id: '00000000-0000-0000-0000-000000000001',
		title: 'RabbitMQ queue backlog on checkout-worker',
		service: 'checkout-worker',
		severity: 'sev1',
		status: 'investigating',
		opened_at: new Date(Date.now() - 18 * 60 * 1000).toISOString(),
		created_at: new Date(Date.now() - 18 * 60 * 1000).toISOString(),
		updated_at: new Date(Date.now() - 2 * 60 * 1000).toISOString()
	},
	{
		id: '40000000-0000-0000-0000-000000000002',
		org_id: '00000000-0000-0000-0000-000000000001',
		title: 'Database connection saturation on payment-api',
		service: 'payment-api',
		severity: 'sev2',
		status: 'open',
		opened_at: new Date(Date.now() - 42 * 60 * 1000).toISOString(),
		created_at: new Date(Date.now() - 42 * 60 * 1000).toISOString(),
		updated_at: new Date(Date.now() - 35 * 60 * 1000).toISOString()
	},
	{
		id: '40000000-0000-0000-0000-000000000003',
		org_id: '00000000-0000-0000-0000-000000000001',
		title: 'Elevated 502 Bad Gateway error spike on auth-api',
		service: 'auth-api',
		severity: 'sev2',
		status: 'mitigated',
		opened_at: new Date(Date.now() - 120 * 60 * 1000).toISOString(),
		created_at: new Date(Date.now() - 120 * 60 * 1000).toISOString(),
		updated_at: new Date(Date.now() - 15 * 60 * 1000).toISOString()
	},
	{
		id: '40000000-0000-0000-0000-000000000004',
		org_id: '00000000-0000-0000-0000-000000000001',
		title: 'Memory leak and OOMKilled restarts on notification-worker',
		service: 'notification-worker',
		severity: 'sev3',
		status: 'resolved',
		opened_at: new Date(Date.now() - 360 * 60 * 1000).toISOString(),
		created_at: new Date(Date.now() - 360 * 60 * 1000).toISOString(),
		updated_at: new Date(Date.now() - 240 * 60 * 1000).toISOString()
	}
];

export const DEMO_RUNS: Record<string, Run> = {
	'40000000-0000-0000-0000-000000000001': {
		id: '50000000-0000-0000-0000-000000000001',
		org_id: '00000000-0000-0000-0000-000000000001',
		incident_id: '40000000-0000-0000-0000-000000000001',
		status: 'succeeded',
		started_at: new Date(Date.now() - 17 * 60 * 1000).toISOString(),
		completed_at: new Date(Date.now() - 15 * 60 * 1000).toISOString(),
		total_tokens: 4320,
		total_cost_usd: 0.001296
	}
};

export const DEMO_RUN_STEPS: Record<string, RunStep[]> = {
	'50000000-0000-0000-0000-000000000001': [
		{
			id: 'step-1',
			org_id: '00000000-0000-0000-0000-000000000001',
			run_id: '50000000-0000-0000-0000-000000000001',
			step_number: 1,
			step_type: 'plan',
			tool_output: {
				content:
					'I am investigating the RabbitMQ queue backlog on checkout-worker. Plan:\n1. Inspect recent releases to determine if a bad commit coincides with the queue accumulation.\n2. Query queue depth metrics to measure messages_ready and consumer throughput rate.'
			},
			tokens_in: 320,
			tokens_out: 140,
			latency_ms: 220,
			created_at: new Date(Date.now() - 17 * 60 * 1000).toISOString()
		},
		{
			id: 'step-2',
			org_id: '00000000-0000-0000-0000-000000000001',
			run_id: '50000000-0000-0000-0000-000000000001',
			step_number: 2,
			step_type: 'tool_call',
			tool_name: 'recent_deploys',
			tool_input: { service: 'checkout-worker', limit: 3 },
			tool_output: {
				service: 'checkout-worker',
				count: 2,
				deployments: [
					{
						id: 'dep-902',
						version: 'v2.14.0',
						commit_hash: '9a7f3e1b',
						deployer: 'alex.dev@corp.internal',
						status: 'deployed',
						deployed_at: new Date(Date.now() - 25 * 60 * 1000).toISOString(),
						message: 'feat(checkout): enable strict synchronous fraud score validation'
					},
					{
						id: 'dep-901',
						version: 'v2.13.9',
						commit_hash: '3c8d1a4e',
						deployer: 'ci-pipeline',
						status: 'stable',
						deployed_at: new Date(Date.now() - 180 * 60 * 1000).toISOString(),
						message: 'fix(checkout): thread pool timeout tuning'
					}
				]
			},
			latency_ms: 38,
			created_at: new Date(Date.now() - 16 * 60 * 1000 + 40000).toISOString()
		},
		{
			id: 'step-3',
			org_id: '00000000-0000-0000-0000-000000000001',
			run_id: '50000000-0000-0000-0000-000000000001',
			step_number: 3,
			step_type: 'tool_call',
			tool_name: 'query_metrics',
			tool_input: { query: 'rabbitmq_queue_messages_ready', service: 'checkout-worker' },
			tool_output: {
				query: 'rabbitmq_queue_messages_ready',
				service: 'checkout-worker',
				summary: {
					min: 12,
					max: 18450,
					avg: 9240,
					latest: 18450,
					points_count: 60
				}
			},
			latency_ms: 45,
			created_at: new Date(Date.now() - 16 * 60 * 1000 + 20000).toISOString()
		},
		{
			id: 'step-4',
			org_id: '00000000-0000-0000-0000-000000000001',
			run_id: '50000000-0000-0000-0000-000000000001',
			step_number: 4,
			step_type: 'tool_call',
			tool_name: 'log_search',
			tool_input: { service: 'checkout-worker', pattern: 'deadline exceeded|connection reset' },
			tool_output: {
				service: 'checkout-worker',
				matches_count: 142,
				sample_lines: [
					'ERROR checkout-worker: fraud_client.go:88: rpc error: code = DeadlineExceeded desc = context deadline exceeded (duration: 5002ms)',
					'ERROR checkout-worker: worker.go:142: consumer thread blocked waiting for fraud service response; threadpool exhausted'
				]
			},
			latency_ms: 62,
			created_at: new Date(Date.now() - 16 * 60 * 1000).toISOString()
		},
		{
			id: 'step-5',
			org_id: '00000000-0000-0000-0000-000000000001',
			run_id: '50000000-0000-0000-0000-000000000001',
			step_number: 5,
			step_type: 'conclusion',
			tool_output: {
				content:
					'Investigation concluded with high certainty. Deployed commit 9a7f3e1b introduced blocking RPC calls to fraud validation service with a 5000ms timeout, starving checkout worker consumer threads and causing an 18,450 message backlog spike.'
			},
			tokens_in: 1200,
			tokens_out: 480,
			latency_ms: 180,
			created_at: new Date(Date.now() - 15 * 60 * 1000).toISOString()
		}
	]
};

export const DEMO_HYPOTHESES: Record<string, HypothesisData[]> = {
	'50000000-0000-0000-0000-000000000001': [
		{
			id: 'hyp-1',
			rank: 1,
			root_cause:
				'Commit 9a7f3e1b deployed 25 minutes ago introduced unbuffered synchronous fraud-check RPC calls with a 5s deadline, causing threadpool starvation and consumer stall on checkout-worker.',
			confidence: 0.94,
			evidence: [
				'Deploy 9a7f3e1b ("feat: strict synchronous fraud score validation") completed 7 minutes before queue backlog onset',
				'rabbitmq_queue_messages_ready surged from 12 to 18,450 messages/sec',
				'142 error logs matching "context deadline exceeded (duration: 5002ms)" from fraud_client.go:88',
				'Downstream fraud-service latency spike confirmed on service_topology dependency graph'
			],
			checklist: [
				'Check downstream fraud-api pod health and error rates',
				'Toggle feature flag `checkout_strict_fraud_check` to false or scale checkout-worker',
				'Execute rolling rollback to deployment v2.13.9 (commit 3c8d1a4e) if fraud-api is unresponsive'
			]
		},
		{
			id: 'hyp-2',
			rank: 2,
			root_cause:
				'RabbitMQ consumer prefetch limit misconfiguration causing unbalanced queue partitioning across available worker pods.',
			confidence: 0.38,
			evidence: [
				'Consumer count remained at 8 across all replicas',
				'No AMQP channel disconnection errors observed in recent logs'
			],
			checklist: [
				'Verify channel QoS prefetch count in checkout-worker configuration',
				'Inspect RabbitMQ cluster memory watermark alarms'
			]
		}
	]
};

export const DEMO_REMEDIATIONS: Record<string, Remediation[]> = {
	'40000000-0000-0000-0000-000000000001': [
		{
			id: 'rem-demo-1',
			org_id: '00000000-0000-0000-0000-000000000001',
			incident_id: '40000000-0000-0000-0000-000000000001',
			action_type: 'scale_service',
			target: 'checkout-worker',
			params: { desired_replicas: 12, previous_replicas: 4 },
			rationale:
				'Triple consumer worker replica count to absorb the 18,450 message queue backlog while fraud service rate limits are investigated.',
			risk_level: 'low',
			expected_effect: 'Queue drain rate increases from 40 msg/s to 450 msg/s within 90 seconds.',
			rollback_plan: 'Scale replicas back to 4 once messages_ready falls below 200.',
			status: 'proposed',
			expires_at: new Date(Date.now() + 12 * 60 * 1000).toISOString(),
			created_at: new Date(Date.now() - 14 * 60 * 1000).toISOString(),
			updated_at: new Date(Date.now() - 14 * 60 * 1000).toISOString()
		},
		{
			id: 'rem-demo-2',
			org_id: '00000000-0000-0000-0000-000000000001',
			incident_id: '40000000-0000-0000-0000-000000000001',
			action_type: 'toggle_feature_flag',
			target: 'checkout_strict_fraud_check',
			params: { enabled: false },
			rationale:
				'Bypass synchronous fraud evaluation RPCs and fallback to asynchronous queue-based validation.',
			risk_level: 'medium',
			expected_effect: 'Worker threadpool blockages eliminated immediately; worker latency drops from 5002ms to 24ms.',
			rollback_plan: 'Re-enable flag checkout_strict_fraud_check once fraud service capacity is restored.',
			status: 'proposed',
			expires_at: new Date(Date.now() + 8 * 60 * 1000).toISOString(),
			created_at: new Date(Date.now() - 13 * 60 * 1000).toISOString(),
			updated_at: new Date(Date.now() - 13 * 60 * 1000).toISOString()
		},
		{
			id: 'rem-demo-3',
			org_id: '00000000-0000-0000-0000-000000000001',
			incident_id: '40000000-0000-0000-0000-000000000001',
			action_type: 'rollback_deploy',
			target: 'checkout-worker',
			params: { target_version: 'v2.13.9', previous_commit: '3c8d1a4e' },
			rationale:
				'Revert deployment v2.14.0 (commit 9a7f3e1b) to restore stable release v2.13.9.',
			risk_level: 'high',
			expected_effect: 'Returns codebase to release prior to synchronous fraud verification.',
			rollback_plan: 'Re-deploy v2.14.0 container tag if regression was unrelated.',
			status: 'proposed',
			expires_at: new Date(Date.now() + 15 * 60 * 1000).toISOString(),
			created_at: new Date(Date.now() - 10 * 60 * 1000).toISOString(),
			updated_at: new Date(Date.now() - 10 * 60 * 1000).toISOString()
		}
	]
};

export const DEMO_REMEDIATION_EVENTS: Record<string, RemediationEvent[]> = {
	'rem-demo-1': [
		{
			id: 'ev-1',
			org_id: '00000000-0000-0000-0000-000000000001',
			remediation_id: 'rem-demo-1',
			transition_from: '',
			transition_to: 'proposed',
			reason: 'Proposed by Autonomous Investigation Agent based on Hypothesis #1',
			created_at: new Date(Date.now() - 14 * 60 * 1000).toISOString()
		}
	]
};

export const DEMO_RUNBOOKS: Runbook[] = [
	{
		id: 'rb-1',
		org_id: '00000000-0000-0000-0000-000000000001',
		title: 'RabbitMQ Consumer Lag & Backlog Mitigation SOP',
		service: 'checkout-worker',
		description: 'Standard operating procedure for message queue congestion and dead-letter backlog draining.',
		content_markdown: `# RabbitMQ Queue Backlog Runbook

### Symptoms
- Metric: \`rabbitmq_queue_messages_ready\` > 5,000
- Consumer utilization dropping below 60%
- Upstream order processing latency increasing

### Diagnostic Steps
1. Inspect consumer connection count:
   \`\`\`bash
   rabbitmqctl list_queues name messages_ready consumers
   \`\`\`
2. Check worker logs for thread deadlocks:
   \`\`\`bash
   kubectl logs -n prod -l app=checkout-worker --tail=200 | grep -E 'timeout|deadlock'
   \`\`\`

### Mitigations
1. **Scale Consumers**: Scale replica deployment up to 12 pods.
2. **Feature Flag**: Deactivate non-essential synchronous enrichment pipelines.
3. **Consumer Restart**: If worker threads are hung on sockets, execute rolling restart:
   \`\`\`bash
   kubectl rollout restart deployment/checkout-worker -n prod
   \`\`\`
`,
		markdown_content: `# RabbitMQ Queue Backlog Runbook

### Symptoms
- Metric: \`rabbitmq_queue_messages_ready\` > 5,000
- Consumer utilization dropping below 60%
- Upstream order processing latency increasing

### Diagnostic Steps
1. Inspect consumer connection count:
   \`\`\`bash
   rabbitmqctl list_queues name messages_ready consumers
   \`\`\`
2. Check worker logs for thread deadlocks:
   \`\`\`bash
   kubectl logs -n prod -l app=checkout-worker --tail=200 | grep -E 'timeout|deadlock'
   \`\`\`

### Mitigations
1. **Scale Consumers**: Scale replica deployment up to 12 pods.
2. **Feature Flag**: Deactivate non-essential synchronous enrichment pipelines.
3. **Consumer Restart**: If worker threads are hung on sockets, execute rolling restart:
   \`\`\`bash
   kubectl rollout restart deployment/checkout-worker -n prod
   \`\`\`
`,
		created_at: '2026-10-09T10:00:00Z',
		updated_at: '2026-10-09T10:00:00Z'
	},
	{
		id: 'rb-2',
		org_id: '00000000-0000-0000-0000-000000000001',
		title: 'Database Connection Pool Saturation SOP',
		service: 'payment-api',
		description: 'Mitigation guidelines for Postgres max_connections exhaustion and connection starvation.',
		content_markdown: `# PostgreSQL Connection Pool Saturation SOP

### Symptoms
- Metric: \`pg_stat_activity_count\` > 95% of \`max_connections\`
- Error: \`remaining connection slots are reserved for non-replication superuser connections\`

### Mitigations
1. Kill idle in transaction queries:
   \`\`\`sql
   SELECT pg_terminate_backend(pid) FROM pg_stat_activity WHERE state = 'idle in transaction' AND state_change < now() - INTERVAL '5 minutes';
   \`\`\`
2. Temporarily increase PgBouncer pool ceiling.
`,
		markdown_content: `# PostgreSQL Connection Pool Saturation SOP

### Symptoms
- Metric: \`pg_stat_activity_count\` > 95% of \`max_connections\`
- Error: \`remaining connection slots are reserved for non-replication superuser connections\`

### Mitigations
1. Kill idle in transaction queries:
   \`\`\`sql
   SELECT pg_terminate_backend(pid) FROM pg_stat_activity WHERE state = 'idle in transaction' AND state_change < now() - INTERVAL '5 minutes';
   \`\`\`
2. Temporarily increase PgBouncer pool ceiling.
`,
		created_at: '2026-10-09T10:00:00Z',
		updated_at: '2026-10-09T10:00:00Z'
	}
];
