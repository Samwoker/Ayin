<script lang="ts">
	import { Card } from '#lib/components/ui/card/index.js';
	import { TriangleAlert, Flame, Bot, ShieldCheck } from '@lucide/svelte';

	interface Props {
		activeCount: number;
		sev1Count: number;
		runsCount: number;
		pendingRemediationsCount: number;
	}

	let {
		activeCount = 0,
		sev1Count = 0,
		runsCount = 0,
		pendingRemediationsCount = 0
	}: Props = $props();
</script>

<div class="grid grid-cols-2 md:grid-cols-4 gap-4">
	<!-- Active Incidents -->
	<Card class="p-4 flex items-center justify-between border-slate-800/80 bg-slate-900/60 hover:border-slate-700/80 hover:bg-slate-900/80 transition-all duration-200 group shadow-sm">
		<div>
			<span class="text-[11px] font-mono uppercase tracking-wider text-slate-400 font-semibold block">Active Incidents</span>
			<div class="text-2xl font-bold font-mono text-white mt-1 group-hover:text-rose-200 transition">{activeCount}</div>
		</div>
		<div class="w-10 h-10 rounded-xl bg-rose-950/60 border border-rose-800/50 flex items-center justify-center text-rose-400 shadow-sm group-hover:scale-105 transition">
			<TriangleAlert class="w-5 h-5" />
		</div>
	</Card>

	<!-- Critical / Sev1 -->
	<Card class="p-4 flex items-center justify-between border-slate-800/80 bg-slate-900/60 hover:border-rose-900/60 hover:bg-slate-900/80 transition-all duration-200 group shadow-sm">
		<div>
			<span class="text-[11px] font-mono uppercase tracking-wider text-rose-400 font-semibold block">Sev1 / Critical</span>
			<div class="text-2xl font-bold font-mono text-rose-400 mt-1">{sev1Count}</div>
		</div>
		<div class="w-10 h-10 rounded-xl bg-red-950/80 border border-red-800/70 flex items-center justify-center text-red-300 shadow-sm group-hover:scale-105 transition">
			<Flame class="w-5 h-5 text-red-400" />
		</div>
	</Card>

	<!-- Autonomous Runs -->
	<Card class="p-4 flex items-center justify-between border-slate-800/80 bg-slate-900/60 hover:border-indigo-900/60 hover:bg-slate-900/80 transition-all duration-200 group shadow-sm">
		<div>
			<span class="text-[11px] font-mono uppercase tracking-wider text-indigo-300 font-semibold block">Autonomous Runs</span>
			<div class="text-2xl font-bold font-mono text-indigo-400 mt-1 group-hover:text-indigo-300 transition">{runsCount}</div>
		</div>
		<div class="w-10 h-10 rounded-xl bg-indigo-950/60 border border-indigo-800/50 flex items-center justify-center text-indigo-400 shadow-sm group-hover:scale-105 transition">
			<Bot class="w-5 h-5" />
		</div>
	</Card>

	<!-- Pending Remediations (HITL Approval Required) -->
	<Card class="p-4 flex items-center justify-between border-slate-800/80 bg-slate-900/60 hover:border-amber-900/60 hover:bg-slate-900/80 transition-all duration-200 group shadow-sm">
		<div>
			<span class="text-[11px] font-mono uppercase tracking-wider text-amber-400 font-semibold block">HITL Approvals</span>
			<div class="text-2xl font-bold font-mono text-amber-400 mt-1 group-hover:text-amber-300 transition">{pendingRemediationsCount}</div>
		</div>
		<div class="w-10 h-10 rounded-xl bg-amber-950/60 border border-amber-800/50 flex items-center justify-center text-amber-400 shadow-sm group-hover:scale-105 transition">
			<ShieldCheck class="w-5 h-5" />
		</div>
	</Card>
</div>
