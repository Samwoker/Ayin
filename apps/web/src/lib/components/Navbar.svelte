<script lang="ts">
	import { Badge } from '#lib/components/ui/badge/index.js';
	import { Button } from '#lib/components/ui/button/index.js';
	import { Radio, Zap, RefreshCw, Server, Activity } from '@lucide/svelte';

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

<header class="border-b border-slate-800/80 bg-slate-950/75 backdrop-blur-md sticky top-0 z-40 px-6 py-3 flex flex-wrap items-center justify-between gap-4 transition-all">
	<!-- Left: Brand & Status -->
	<div class="flex items-center space-x-3.5">
		<div class="relative w-9 h-9 rounded-xl bg-gradient-to-br from-rose-500 via-red-600 to-rose-700 flex items-center justify-center font-mono font-bold text-white shadow-lg shadow-red-950/40 ring-1 ring-white/20">
			<span>T</span>
			<span class="absolute -bottom-0.5 -right-0.5 w-2.5 h-2.5 rounded-full bg-emerald-500 ring-2 ring-slate-950"></span>
		</div>
		<div>
			<div class="flex items-center gap-2">
				<h1 class="text-base font-bold tracking-tight text-white font-sans flex items-center gap-1.5">
					<span>triage</span>
				</h1>
				<Badge variant="sev1" class="text-[10px] px-2 py-0 animate-none font-mono">
					v0.1.0
				</Badge>
			</div>
			<p class="text-[11px] text-slate-400 font-sans">Autonomous Incident Response & HITL Remediation Engine</p>
		</div>
	</div>

	<!-- Center / Right: Organization & Real-time Connectivity -->
	<div class="flex items-center gap-3 text-xs">
		<!-- Tenant Org Selector -->
		<div class="flex items-center gap-2 px-3 py-1.5 rounded-lg bg-slate-900/90 border border-slate-800 text-slate-300 font-medium shadow-sm">
			<Server class="w-3.5 h-3.5 text-slate-400" />
			<span class="text-slate-400 font-mono text-[11px]">Org:</span>
			<span class="text-white font-semibold flex items-center gap-1.5 font-mono text-xs">
				<span class="w-1.5 h-1.5 rounded-full bg-blue-400"></span>
				{activeTenant}
			</span>
		</div>

		<!-- Live Backend Connection Pill -->
		<div class="flex items-center gap-2 px-3 py-1.5 rounded-lg border font-mono text-[11px] {backendConnected ? 'bg-emerald-950/50 border-emerald-800/70 text-emerald-300 shadow-sm shadow-emerald-950/20' : 'bg-amber-950/50 border-amber-800/70 text-amber-300'}">
			{#if backendConnected}
				<span class="relative flex h-2 w-2">
					<span class="animate-ping absolute inline-flex h-full w-full rounded-full bg-emerald-400 opacity-75"></span>
					<span class="relative inline-flex rounded-full h-2 w-2 bg-emerald-500"></span>
				</span>
				<Activity class="w-3.5 h-3.5 text-emerald-400" />
				<span>Live API ({latencyMs}ms)</span>
			{:else}
				<Radio class="w-3.5 h-3.5 text-amber-400" />
				<span>Standalone / Mock</span>
			{/if}
		</div>

		<!-- Action: Trigger Fault Simulation -->
		<Button
			variant="destructive"
			size="sm"
			onclick={onSimulate}
			class="bg-red-950/80 hover:bg-red-900 border border-red-800/80 text-red-200 gap-1.5 shadow-sm active:scale-95 transition"
			title="Inject a test alert into the triage pipeline"
		>
			<Zap class="w-3.5 h-3.5 text-rose-400" />
			<span>Simulate Alert</span>
		</Button>

		<!-- Refresh / Sync Action -->
		<Button
			variant="secondary"
			size="sm"
			onclick={onRefresh}
			disabled={isRefreshing}
			class="border border-slate-700/80 bg-slate-900/90 text-slate-200 hover:text-white gap-1.5 shadow-sm active:scale-95 transition"
			title="Refresh incidents and runs"
		>
			<RefreshCw class="w-3.5 h-3.5 {isRefreshing ? 'animate-spin' : ''}" />
			<span>{isRefreshing ? 'Syncing...' : 'Sync'}</span>
		</Button>
	</div>
</header>
