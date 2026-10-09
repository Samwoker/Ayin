<script lang="ts">
	import { onMount } from 'svelte';
	import { apiClient, type SystemOverview } from '#lib';

	let system = $state<SystemOverview>({
		connected: false,
		latencyMs: 0,
		lastChecked: 'Never'
	});
	let isRefreshing = $state(false);
	let autoRefresh = $state(true);
	let showRawJson = $state(false);

	async function refreshStatus() {
		isRefreshing = true;
		try {
			const overview = await apiClient.checkSystemOverview();
			system = overview;
		} catch (err: unknown) {
			system = {
				connected: false,
				latencyMs: 0,
				lastChecked: new Date().toISOString(),
				errorMessage: err instanceof Error ? err.message : String(err)
			};
		} finally {
			isRefreshing = false;
		}
	}

	onMount(() => {
		refreshStatus();

		const interval = setInterval(() => {
			if (autoRefresh) {
				refreshStatus();
			}
		}, 5000);

		return () => clearInterval(interval);
	});
</script>

<svelte:head>
	<title>triage — Autonomous Incident Triage Agent</title>
</svelte:head>

<div class="min-h-screen bg-neutral-950 text-neutral-100 font-sans selection:bg-red-500/20 selection:text-red-400">
	<!-- Top Navigation -->
	<header class="border-b border-neutral-800 bg-neutral-900/60 backdrop-blur sticky top-0 z-50 px-6 py-4 flex flex-wrap items-center justify-between gap-4">
		<div class="flex items-center space-x-3">
			<div class="w-9 h-9 rounded-lg bg-red-600/20 border border-red-500/40 flex items-center justify-center font-mono font-bold text-red-400 shadow-inner">
				T
			</div>
			<div>
				<h1 class="text-lg font-semibold tracking-tight text-white flex items-center gap-2">
					triage
					<span class="text-xs px-2 py-0.5 rounded-full bg-red-950/80 border border-red-800/60 text-red-400 font-mono">
						v0.1.0-alpha
					</span>
				</h1>
				<p class="text-xs text-neutral-400">Autonomous Incident Investigation & Root-Cause Agent</p>
			</div>
		</div>

		<!-- Real-time Backend Connection Badges -->
		<div class="flex items-center gap-3 text-xs font-mono">
			<!-- Backend Connection Status -->
			<div class="flex items-center gap-2 px-3 py-1.5 rounded-md border {system.connected ? 'bg-emerald-950/40 border-emerald-800/60 text-emerald-300' : 'bg-red-950/40 border-red-800/60 text-red-300'}">
				<span class="relative flex h-2 w-2">
					{#if system.connected}
						<span class="animate-ping absolute inline-flex h-full w-full rounded-full bg-emerald-400 opacity-75"></span>
						<span class="relative inline-flex rounded-full h-2 w-2 bg-emerald-500"></span>
					{:else}
						<span class="relative inline-flex rounded-full h-2 w-2 bg-red-500"></span>
					{/if}
				</span>
				<span>Backend: {system.connected ? 'Connected' : 'Offline'}</span>
				{#if system.connected}
					<span class="text-[10px] text-neutral-400">({system.latencyMs}ms)</span>
				{/if}
			</div>

			<!-- Environment Badge -->
			<div class="hidden sm:flex items-center gap-1.5 px-2.5 py-1.5 rounded-md bg-neutral-900 border border-neutral-800 text-neutral-400">
				<span>Env:</span>
				<span class="text-neutral-200">{system.status?.environment || 'development'}</span>
			</div>

			<!-- Ping / Refresh Action -->
			<button
				onclick={refreshStatus}
				disabled={isRefreshing}
				class="flex items-center gap-1.5 px-3 py-1.5 rounded-md bg-neutral-800 hover:bg-neutral-700 text-neutral-200 transition cursor-pointer disabled:opacity-50"
				title="Ping Go API"
			>
				<span class="{isRefreshing ? 'animate-spin' : ''}">⟳</span>
				<span>{isRefreshing ? 'Pinging...' : 'Ping API'}</span>
			</button>
		</div>
	</header>

	<!-- Main Content Area -->
	<main class="max-w-7xl mx-auto px-6 py-8 space-y-8">
		<!-- Integration Status Banner -->
		{#if !system.connected}
			<div class="rounded-xl border border-red-800/60 bg-red-950/20 p-5 flex flex-col sm:flex-row items-start sm:items-center justify-between gap-4">
				<div class="space-y-1">
					<h2 class="text-sm font-semibold text-red-300 flex items-center gap-2">
						<span class="inline-block w-2 h-2 rounded-full bg-red-500"></span>
						Backend API Disconnected
					</h2>
					<p class="text-xs text-neutral-400">
						The SvelteKit frontend is attempting to reach the Go backend at <code class="text-neutral-300 font-mono">http://localhost:8080</code>.
						{system.errorMessage ? `(${system.errorMessage})` : ''}
					</p>
				</div>
				<div class="text-xs font-mono bg-neutral-900 border border-neutral-800 px-3 py-2 rounded text-neutral-300 whitespace-nowrap">
					Run: <span class="text-red-400 font-semibold">make dev</span> or <span class="text-red-400 font-semibold">./apps/api/bin/api</span>
				</div>
			</div>
		{:else}
			<div class="rounded-xl border border-emerald-900/40 bg-emerald-950/10 p-5 flex flex-col sm:flex-row items-start sm:items-center justify-between gap-4">
				<div class="space-y-1">
					<h2 class="text-sm font-semibold text-emerald-300 flex items-center gap-2">
						<span class="inline-block w-2 h-2 rounded-full bg-emerald-400"></span>
						Full-Stack Integration Active
					</h2>
					<p class="text-xs text-neutral-400">
						Go backend (<span class="text-neutral-300 font-mono">{system.status?.service}</span>) is responding to health, readiness, and status probes with <span class="text-emerald-400 font-mono font-semibold">{system.latencyMs}ms</span> latency.
					</p>
				</div>
				<div class="text-xs font-mono text-neutral-400 flex items-center gap-2">
					<span>Uptime:</span>
					<span class="text-white font-semibold">{system.status?.uptime_seconds}s</span>
				</div>
			</div>
		{/if}

		<!-- Hero Information Card -->
		<div class="rounded-xl border border-neutral-800 bg-neutral-900/40 p-8 backdrop-blur relative overflow-hidden">
			<div class="absolute -right-20 -bottom-20 w-80 h-80 rounded-full bg-red-600/5 blur-3xl pointer-events-none"></div>
			<div class="max-w-2xl">
				<span class="text-xs font-mono uppercase tracking-wider text-red-400 font-semibold">Phase 1 Integration Verified</span>
				<h2 class="text-2xl font-bold text-white mt-2 tracking-tight">Autonomous Incident Triage Control Plane</h2>
				<p class="text-neutral-400 text-sm mt-2 leading-relaxed">
					Ingests alerts and telemetry, deploys read-only investigative tools, formulates ranked root-cause hypotheses with verified evidence links, and generates actionable first-response checklists.
				</p>
			</div>
		</div>

		<!-- Diagnostic Probe Cards -->
		<div class="grid grid-cols-1 md:grid-cols-3 gap-6">
			<!-- Liveness Probe Card -->
			<div class="rounded-lg border border-neutral-800 bg-neutral-900/30 p-5">
				<div class="flex items-center justify-between mb-3">
					<span class="text-xs font-mono text-neutral-400">GET /healthz</span>
					<span class="px-2 py-0.5 text-xs font-mono rounded {system.health?.status === 'ok' ? 'bg-emerald-950 border border-emerald-800/60 text-emerald-400' : 'bg-neutral-800 text-neutral-500'}">
						{system.health?.status || 'unreachable'}
					</span>
				</div>
				<h3 class="text-sm font-semibold text-white">Liveness Probe</h3>
				<p class="text-xs text-neutral-400 mt-1">HTTP server health & process liveliness</p>
				<div class="mt-4 pt-3 border-t border-neutral-800/60 flex items-center justify-between text-[11px] font-mono text-neutral-500">
					<span>Service:</span>
					<span class="text-neutral-300">{system.health?.service || '—'}</span>
				</div>
			</div>

			<!-- Readiness Probe Card -->
			<div class="rounded-lg border border-neutral-800 bg-neutral-900/30 p-5">
				<div class="flex items-center justify-between mb-3">
					<span class="text-xs font-mono text-neutral-400">GET /readyz</span>
					<span class="px-2 py-0.5 text-xs font-mono rounded {system.ready?.status === 'ready' ? 'bg-emerald-950 border border-emerald-800/60 text-emerald-400' : 'bg-neutral-800 text-neutral-500'}">
						{system.ready?.status || 'unreachable'}
					</span>
				</div>
				<h3 class="text-sm font-semibold text-white">Readiness Probe</h3>
				<p class="text-xs text-neutral-400 mt-1">Downstream dependencies & queue status</p>
				<div class="mt-4 pt-3 border-t border-neutral-800/60 flex items-center justify-between text-[11px] font-mono text-neutral-500">
					<span>Checks:</span>
					<span class="text-neutral-300">{system.ready?.checks ? Object.keys(system.ready.checks).length : 0} configured</span>
				</div>
			</div>

			<!-- API Status Card -->
			<div class="rounded-lg border border-neutral-800 bg-neutral-900/30 p-5">
				<div class="flex items-center justify-between mb-3">
					<span class="text-xs font-mono text-neutral-400">GET /api/v1/status</span>
					<span class="px-2 py-0.5 text-xs font-mono rounded {system.status?.status === 'online' ? 'bg-emerald-950 border border-emerald-800/60 text-emerald-400' : 'bg-neutral-800 text-neutral-500'}">
						{system.status?.status || 'unreachable'}
					</span>
				</div>
				<h3 class="text-sm font-semibold text-white">System Status</h3>
				<p class="text-xs text-neutral-400 mt-1">Version, uptime & telemetry metadata</p>
				<div class="mt-4 pt-3 border-t border-neutral-800/60 flex items-center justify-between text-[11px] font-mono text-neutral-500">
					<span>Version:</span>
					<span class="text-neutral-300 font-semibold">{system.status?.version || '0.1.0'}</span>
				</div>
			</div>
		</div>

		<!-- Interactive Inspector Drawer -->
		<div class="rounded-lg border border-neutral-800 bg-neutral-900/20 p-6 space-y-4">
			<div class="flex items-center justify-between flex-wrap gap-2">
				<div>
					<h3 class="text-sm font-semibold text-white tracking-wide uppercase font-mono text-neutral-300">
						Live Backend Response Inspector
					</h3>
					<p class="text-xs text-neutral-500">
						Last polled: <span class="font-mono text-neutral-400">{system.lastChecked}</span>
					</p>
				</div>
				<div class="flex items-center gap-3 text-xs font-mono">
					<label class="flex items-center gap-2 cursor-pointer text-neutral-400 hover:text-neutral-200">
						<input type="checkbox" bind:checked={autoRefresh} class="accent-red-500 rounded" />
						<span>Auto-poll (5s)</span>
					</label>
					<button
						onclick={() => (showRawJson = !showRawJson)}
						class="px-2.5 py-1 rounded bg-neutral-800 hover:bg-neutral-700 text-neutral-300 transition cursor-pointer"
					>
						{showRawJson ? 'Hide JSON' : 'View Raw Payload'}
					</button>
				</div>
			</div>

			{#if showRawJson}
				<pre class="p-4 rounded-lg bg-neutral-950 border border-neutral-800 font-mono text-xs text-emerald-400 overflow-x-auto max-h-72">{JSON.stringify(system, null, 2)}</pre>
			{/if}
		</div>
	</main>
</div>
