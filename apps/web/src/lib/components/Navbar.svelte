<script lang="ts">
	interface Props {
		backendConnected: boolean;
		latencyMs: number;
		activeTenant: string;
		onSimulate: () => void;
		onRefresh: () => void;
		isRefreshing: boolean;
	}

	let {
		backendConnected = false,
		latencyMs = 0,
		activeTenant = 'Acme Corp (Primary)',
		onSimulate,
		onRefresh,
		isRefreshing = false
	}: Props = $props();
</script>

<header class="border-b border-neutral-800/80 bg-neutral-900/70 backdrop-blur-md sticky top-0 z-40 px-6 py-3.5 flex flex-wrap items-center justify-between gap-4">
	<!-- Left: Brand & Status -->
	<div class="flex items-center space-x-3.5">
		<div class="w-9 h-9 rounded-xl bg-gradient-to-br from-red-500 to-rose-700 flex items-center justify-center font-mono font-bold text-white shadow-lg shadow-red-950/50 ring-1 ring-white/20">
			T
		</div>
		<div>
			<div class="flex items-center gap-2">
				<h1 class="text-base font-bold tracking-tight text-white font-sans">
					triage
				</h1>
				<span class="text-[11px] px-2 py-0.5 rounded-full bg-red-950/60 border border-red-800/60 text-red-400 font-mono font-medium">
					v0.1.0
				</span>
			</div>
			<p class="text-[11px] text-neutral-400">Autonomous Incident Response & HITL Remediation Engine</p>
		</div>
	</div>

	<!-- Center / Right: Organization & Real-time Connectivity -->
	<div class="flex items-center gap-3 text-xs">
		<!-- Tenant Org Selector -->
		<div class="flex items-center gap-2 px-3 py-1.5 rounded-lg bg-neutral-950 border border-neutral-800 text-neutral-300 font-medium">
			<span class="text-neutral-500">Org:</span>
			<span class="text-white font-semibold flex items-center gap-1.5">
				<span class="w-2 h-2 rounded-full bg-blue-500"></span>
				{activeTenant}
			</span>
		</div>

		<!-- Live Backend Connection Pill -->
		<div class="flex items-center gap-2 px-3 py-1.5 rounded-lg border font-mono text-[11px] {backendConnected ? 'bg-emerald-950/40 border-emerald-800/60 text-emerald-300' : 'bg-amber-950/40 border-amber-800/60 text-amber-300'}">
			<span class="relative flex h-2 w-2">
				{#if backendConnected}
					<span class="animate-ping absolute inline-flex h-full w-full rounded-full bg-emerald-400 opacity-75"></span>
					<span class="relative inline-flex rounded-full h-2 w-2 bg-emerald-500"></span>
				{:else}
					<span class="relative inline-flex rounded-full h-2 w-2 bg-amber-500"></span>
				{/if}
			</span>
			<span>{backendConnected ? `Live API (${latencyMs}ms)` : 'Standalone / Mock Mode'}</span>
		</div>

		<!-- Action: Trigger Fault Simulation -->
		<button
			onclick={onSimulate}
			class="flex items-center gap-1.5 px-3 py-1.5 rounded-lg bg-red-900/40 hover:bg-red-800/50 border border-red-700/60 text-red-200 transition font-medium cursor-pointer shadow-sm active:scale-95"
			title="Inject a test alert into the triage pipeline"
		>
			<span>⚡</span>
			<span>Simulate Alert</span>
		</button>

		<!-- Refresh / Sync Action -->
		<button
			onclick={onRefresh}
			disabled={isRefreshing}
			class="flex items-center gap-1.5 px-3 py-1.5 rounded-lg bg-neutral-800 hover:bg-neutral-700 border border-neutral-700 text-neutral-200 transition font-medium cursor-pointer disabled:opacity-50 active:scale-95"
			title="Refresh incidents and runs"
		>
			<span class={isRefreshing ? 'animate-spin inline-block' : ''}>⟳</span>
			<span>{isRefreshing ? 'Syncing...' : 'Sync'}</span>
		</button>
	</div>
</header>
