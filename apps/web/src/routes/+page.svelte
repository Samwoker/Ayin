<script lang="ts">
	import { onMount } from 'svelte';
	import { apiClient } from '#lib';
	import type {
		Incident,
		Run,
		RunStep,
		Remediation,
		RemediationEvent,
		Runbook
	} from '#lib/api/types';
	import {
		DEMO_INCIDENTS,
		DEMO_RUNS,
		DEMO_RUN_STEPS,
		DEMO_HYPOTHESES,
		DEMO_REMEDIATIONS,
		DEMO_REMEDIATION_EVENTS,
		DEMO_RUNBOOKS,
		type HypothesisData
	} from '#lib/demo-data';

	import Navbar from '#lib/components/Navbar.svelte';
	import StatsBar from '#lib/components/StatsBar.svelte';
	import IncidentList from '#lib/components/IncidentList.svelte';
	import IncidentDetail from '#lib/components/IncidentDetail.svelte';

	// Reactive Runes State
	let backendConnected = $state(false);
	let latencyMs = $state(0);
	let isRefreshing = $state(false);
	let isTriggeringRun = $state(false);
	let isLiveStreaming = $state(false);
	let activeEventSource: EventSource | null = null;

	let incidents = $state<Incident[]>(DEMO_INCIDENTS);
	let selectedIncidentId = $state<string>(DEMO_INCIDENTS[0].id);

	let selectedIncident = $derived(
		incidents.find((i) => i.id === selectedIncidentId) || incidents[0]
	);

	let runsMap = $state<Record<string, Run>>(DEMO_RUNS);
	let stepsMap = $state<Record<string, RunStep[]>>(DEMO_RUN_STEPS);
	let hypothesesMap = $state<Record<string, HypothesisData[]>>(DEMO_HYPOTHESES);
	let remediationsMap = $state<Record<string, Remediation[]>>(DEMO_REMEDIATIONS);
	let eventsMap = $state<Record<string, RemediationEvent[]>>(DEMO_REMEDIATION_EVENTS);
	let runbooks = $state<Runbook[]>(DEMO_RUNBOOKS);

	let activeRun = $derived(
		selectedIncident ? runsMap[selectedIncident.id] || null : null
	);

	let activeSteps = $derived(
		activeRun ? stepsMap[activeRun.id] || [] : []
	);

	let activeHypotheses = $derived(
		activeRun ? hypothesesMap[activeRun.id] || [] : []
	);

	let activeRemediations = $derived(
		selectedIncident ? remediationsMap[selectedIncident.id] || [] : []
	);

	// Derived metrics
	let activeIncidentsCount = $derived(
		incidents.filter((i) => i.status === 'open' || i.status === 'investigating').length
	);

	let sev1Count = $derived(
		incidents.filter((i) => i.severity === 'sev1').length
	);

	let totalRunsCount = $derived(Object.keys(runsMap).length);

	let pendingRemediationsCount = $derived(
		Object.values(remediationsMap)
			.flat()
			.filter((r) => r.status === 'proposed').length
	);

	async function checkBackendConnectivity() {
		try {
			const overview = await apiClient.checkSystemOverview();
			backendConnected = overview.connected;
			latencyMs = overview.latencyMs;
		} catch {
			backendConnected = false;
			latencyMs = 0;
		}
	}

	async function syncIncidents() {
		isRefreshing = true;
		try {
			await checkBackendConnectivity();
			if (backendConnected) {
				const res = await apiClient.listIncidents();
				if (res && res.incidents && res.incidents.length > 0) {
					incidents = res.incidents;
				}
				const rbs = await apiClient.listRunbooks();
				if (Array.isArray(rbs) && rbs.length > 0) {
					runbooks = rbs;
				}
			}
		} catch (e) {
			console.warn('Backend sync failed, maintaining current state:', e);
		} finally {
			isRefreshing = false;
		}
	}

	function handleSelectIncident(inc: Incident) {
		selectedIncidentId = inc.id;
		setupSSEStream(inc.id);
	}

	function setupSSEStream(incidentId: string) {
		if (activeEventSource) {
			activeEventSource.close();
			activeEventSource = null;
			isLiveStreaming = false;
		}

		const r = runsMap[incidentId];
		if (!r || !backendConnected) return;

		try {
			const url = `http://localhost:8080/api/v1/runs/${r.id}/stream`;
			activeEventSource = new EventSource(url);
			isLiveStreaming = true;

			activeEventSource.addEventListener('step_completed', (e) => {
				try {
					const data = JSON.parse(e.data);
					if (data.step_number && r) {
						const curSteps = stepsMap[r.id] || [];
						stepsMap[r.id] = [
							...curSteps,
							{
								id: `sse-step-${data.step_number}`,
								org_id: r.org_id,
								run_id: r.id,
								step_number: data.step_number,
								step_type: data.step_type || 'tool_call',
								tool_name: data.tool_name,
								latency_ms: data.latency_ms || 35,
								created_at: new Date().toISOString()
							}
						];
					}
				} catch (err) {
					console.error('SSE step parse error:', err);
				}
			});

			activeEventSource.addEventListener('run_completed', () => {
				isLiveStreaming = false;
				if (activeEventSource) {
					activeEventSource.close();
					activeEventSource = null;
				}
			});

			activeEventSource.onerror = () => {
				isLiveStreaming = false;
			};
		} catch {
			isLiveStreaming = false;
		}
	}

	async function handleStatusChange(newStatus: string) {
		if (!selectedIncident) return;
		const incId = selectedIncident.id;

		// Optimistic update
		incidents = incidents.map((i) =>
			i.id === incId ? { ...i, status: newStatus as any, updated_at: new Date().toISOString() } : i
		);

		if (backendConnected) {
			try {
				await apiClient.updateIncidentStatus(incId, { status: newStatus });
			} catch (err) {
				console.error('Failed to update incident status on backend:', err);
			}
		}
	}

	async function handleTriggerRun() {
		if (!selectedIncident) return;
		isTriggeringRun = true;

		try {
			if (backendConnected) {
				const res = await apiClient.createRun(selectedIncident.id);
				runsMap[selectedIncident.id] = res;
				stepsMap[res.id] = [];
				setupSSEStream(selectedIncident.id);
			} else {
				// Standalone simulation: simulate multi-turn steps
				await new Promise((r) => setTimeout(r, 600));
				const mockRunId = `run-sim-${Date.now()}`;
				const mockRun: Run = {
					id: mockRunId,
					org_id: selectedIncident.org_id,
					incident_id: selectedIncident.id,
					status: 'succeeded',
					started_at: new Date().toISOString(),
					completed_at: new Date(Date.now() + 8000).toISOString(),
					total_tokens: 3840,
					total_cost_usd: 0.00115
				};

				runsMap[selectedIncident.id] = mockRun;
				stepsMap[mockRunId] = DEMO_RUN_STEPS['50000000-0000-0000-0000-000000000001'] || [];
				hypothesesMap[mockRunId] = DEMO_HYPOTHESES['50000000-0000-0000-0000-000000000001'] || [];
			}
		} finally {
			isTriggeringRun = false;
		}
	}

	async function handleApproveRemediation(rem: Remediation, reason: string) {
		if (!selectedIncident) return;

		// Optimistic update
		remediationsMap[selectedIncident.id] = (remediationsMap[selectedIncident.id] || []).map((r) =>
			r.id === rem.id ? { ...r, status: 'succeeded', updated_at: new Date().toISOString() } : r
		);

		eventsMap[rem.id] = [
			...(eventsMap[rem.id] || []),
			{
				id: `ev-${Date.now()}`,
				org_id: rem.org_id,
				remediation_id: rem.id,
				transition_from: rem.status,
				transition_to: 'succeeded',
				reason: reason || 'Approved & Executed by responder',
				created_at: new Date().toISOString()
			}
		];

		if (backendConnected) {
			try {
				await apiClient.approveRemediation(rem.id, { reason });
			} catch (e) {
				console.error('Failed to approve remediation on API:', e);
			}
		}
	}

	async function handleRejectRemediation(rem: Remediation, reason: string) {
		if (!selectedIncident) return;

		remediationsMap[selectedIncident.id] = (remediationsMap[selectedIncident.id] || []).map((r) =>
			r.id === rem.id ? { ...r, status: 'rejected', updated_at: new Date().toISOString() } : r
		);

		eventsMap[rem.id] = [
			...(eventsMap[rem.id] || []),
			{
				id: `ev-${Date.now()}`,
				org_id: rem.org_id,
				remediation_id: rem.id,
				transition_from: rem.status,
				transition_to: 'rejected',
				reason: reason || 'Rejected by responder',
				created_at: new Date().toISOString()
			}
		];

		if (backendConnected) {
			try {
				await apiClient.rejectRemediation(rem.id, { reason });
			} catch (e) {
				console.error('Failed to reject remediation on API:', e);
			}
		}
	}

	async function handleRollbackRemediation(rem: Remediation, reason: string) {
		if (!selectedIncident) return;

		remediationsMap[selectedIncident.id] = (remediationsMap[selectedIncident.id] || []).map((r) =>
			r.id === rem.id ? { ...r, status: 'rolled_back', updated_at: new Date().toISOString() } : r
		);

		eventsMap[rem.id] = [
			...(eventsMap[rem.id] || []),
			{
				id: `ev-${Date.now()}`,
				org_id: rem.org_id,
				remediation_id: rem.id,
				transition_from: 'succeeded',
				transition_to: 'rolled_back',
				reason: reason || 'Manual rollback executed',
				created_at: new Date().toISOString()
			}
		];

		if (backendConnected) {
			try {
				await apiClient.rollbackRemediation(rem.id, { reason });
			} catch (e) {
				console.error('Failed to rollback remediation on API:', e);
			}
		}
	}

	async function handleProposeRemediation(payload: {
		action_type: string;
		target: string;
		rationale: string;
		risk_level: string;
	}) {
		if (!selectedIncident) return;

		const newRem: Remediation = {
			id: `rem-user-${Date.now()}`,
			org_id: selectedIncident.org_id,
			incident_id: selectedIncident.id,
			action_type: payload.action_type as any,
			target: payload.target,
			params: {},
			rationale: payload.rationale,
			risk_level: payload.risk_level as any,
			expected_effect: 'Targeted metric returns within baseline threshold.',
			rollback_plan: 'Reverse action parameter.',
			status: 'proposed',
			expires_at: new Date(Date.now() + 15 * 60 * 1000).toISOString(),
			created_at: new Date().toISOString(),
			updated_at: new Date().toISOString()
		};

		remediationsMap[selectedIncident.id] = [
			...(remediationsMap[selectedIncident.id] || []),
			newRem
		];

		if (backendConnected) {
			try {
				await apiClient.proposeRemediation(selectedIncident.id, {
					action_type: payload.action_type as any,
					target: payload.target,
					rationale: payload.rationale,
					risk_level: payload.risk_level as any,
					expected_effect: newRem.expected_effect,
					rollback_plan: newRem.rollback_plan,
					ttl_seconds: 900
				});
			} catch (e) {
				console.error('Failed to propose remediation to API:', e);
			}
		}
	}

	function handleSimulateAlert() {
		const services = ['payment-api', 'checkout-worker', 'auth-api', 'inventory-db', 'notification-worker'];
		const randomService = services[Math.floor(Math.random() * services.length)];
		const newIncident: Incident = {
			id: `sim-${Date.now()}`,
			org_id: '00000000-0000-0000-0000-000000000001',
			title: `Spike in 5xx errors and connection reset on ${randomService}`,
			service: randomService,
			severity: 'sev1',
			status: 'investigating',
			opened_at: new Date().toISOString(),
			created_at: new Date().toISOString(),
			updated_at: new Date().toISOString()
		};

		incidents = [newIncident, ...incidents];
		selectedIncidentId = newIncident.id;
	}

	onMount(() => {
		syncIncidents();
		const interval = setInterval(checkBackendConnectivity, 10000);
		return () => {
			clearInterval(interval);
			if (activeEventSource) {
				activeEventSource.close();
			}
		};
	});
</script>

<svelte:head>
	<title>triage — Incident Response & Remediation Console</title>
</svelte:head>

<div class="min-h-screen bg-neutral-950 text-neutral-100 font-sans selection:bg-red-500/20 selection:text-red-400">
	<!-- Navbar -->
	<Navbar
		{backendConnected}
		{latencyMs}
		activeTenant="Acme Corp (Primary)"
		onSimulate={handleSimulateAlert}
		onRefresh={syncIncidents}
		{isRefreshing}
	/>

	<!-- Main Workspace Layout -->
	<main class="max-w-7xl mx-auto px-4 sm:px-6 py-6 space-y-6">
		<!-- Summary Metrics Bar -->
		<StatsBar
			activeCount={activeIncidentsCount}
			{sev1Count}
			runsCount={totalRunsCount}
			{pendingRemediationsCount}
		/>

		<!-- Master-Detail 2-Column Responsive Console -->
		<div class="grid grid-cols-1 lg:grid-cols-12 gap-6 items-start">
			<!-- Left: Incident Feed & Triage Queue (4 cols) -->
			<div class="lg:col-span-4 sticky top-24">
				<IncidentList
					{incidents}
					{selectedIncidentId}
					onSelect={handleSelectIncident}
				/>
			</div>

			<!-- Right: Incident Command & Investigation Console (8 cols) -->
			<div class="lg:col-span-8">
				{#if selectedIncident}
					<IncidentDetail
						incident={selectedIncident}
						run={activeRun}
						steps={activeSteps}
						hypotheses={activeHypotheses}
						remediations={activeRemediations}
						remediationEvents={eventsMap}
						{runbooks}
						{isLiveStreaming}
						onStatusChange={handleStatusChange}
						onTriggerRun={handleTriggerRun}
						onApproveRemediation={handleApproveRemediation}
						onRejectRemediation={handleRejectRemediation}
						onRollbackRemediation={handleRollbackRemediation}
						onProposeRemediation={handleProposeRemediation}
						isTriggering={isTriggeringRun}
					/>
				{:else}
					<div class="rounded-xl border border-neutral-800 bg-neutral-900/40 p-12 text-center text-neutral-500 font-mono text-xs">
						Select an incident from the queue to start investigation
					</div>
				{/if}
			</div>
		</div>
	</main>
</div>
