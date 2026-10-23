<script lang="ts">
	import {
		AlertCircle,
		Flame,
		Cpu,
		ShieldCheck,
		Activity,
		Command,
		Search,
		CheckCircle2
	} from '@lucide/svelte';

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

<!-- Command Cockpit Header Bar -->
<div class="rounded-xl border border-white/[0.08] bg-[#0d0f14]/90 backdrop-blur-md p-3.5 shadow-lg shadow-black/40">
	<div class="flex items-center justify-between gap-4 flex-wrap">
		<!-- Left: Real-time System Operational State -->
		<div class="flex items-center gap-3">
			<div class="relative flex items-center justify-center w-8 h-8 rounded-lg {sev1Count > 0 ? 'bg-rose-500/10 border border-rose-500/30 text-rose-400' : 'bg-emerald-500/10 border border-emerald-500/30 text-emerald-400'}">
				{#if sev1Count > 0}
					<span class="absolute inset-0 rounded-lg bg-rose-500/20 animate-radar"></span>
					<Flame class="w-4 h-4 text-rose-400 relative z-10" />
				{:else}
					<CheckCircle2 class="w-4 h-4 text-emerald-400 relative z-10" />
				{/if}
			</div>

			<div>
				<div class="flex items-center gap-2">
					<span class="font-mono text-xs font-bold uppercase tracking-wider {sev1Count > 0 ? 'text-rose-400' : 'text-emerald-400'}">
						{sev1Count > 0 ? 'Active Sev1 Outage in Progress' : 'Systems Nominal'}
					</span>
					<span class="text-[10px] font-mono text-slate-500 border border-white/10 rounded px-1.5 py-0.2 bg-white/[0.03]">
						Ayin 👁️ Agent Active
					</span>
				</div>
				<p class="text-[11px] text-slate-400 font-sans">
					{sev1Count > 0 ? 'Autonomous agent correlating commits with telemetry traces' : 'All background health probes and queues operational'}
				</p>
			</div>
		</div>

		<!-- Center: Density Metrics Gauges -->
		<div class="flex items-center gap-3 font-mono text-xs flex-wrap">
			<!-- Active Incidents Pill -->
			<div class="flex items-center gap-2 px-3 py-1.5 rounded-lg bg-black/40 border border-white/[0.06]">
				<span class="text-slate-400 text-[11px]">Incidents:</span>
				<span class="text-white font-bold">{activeCount}</span>
				{#if sev1Count > 0}
					<span class="text-[10px] px-1.5 py-0.2 rounded bg-rose-950/80 border border-rose-800/60 text-rose-300 font-semibold">
						{sev1Count} Sev1
					</span>
				{/if}
			</div>

			<!-- Autonomous Runs -->
			<div class="flex items-center gap-2 px-3 py-1.5 rounded-lg bg-black/40 border border-white/[0.06]">
				<Cpu class="w-3.5 h-3.5 text-indigo-400" />
				<span class="text-slate-400 text-[11px]">Agent Runs:</span>
				<span class="text-indigo-300 font-bold">{runsCount}</span>
			</div>

			<!-- HITL Pending Approvals -->
			<div class="flex items-center gap-2 px-3 py-1.5 rounded-lg bg-black/40 border border-white/[0.06]">
				<ShieldCheck class="w-3.5 h-3.5 text-amber-400" />
				<span class="text-slate-400 text-[11px]">HITL Queue:</span>
				<span class="text-amber-400 font-bold">{pendingRemediationsCount} actions</span>
			</div>

			<!-- Accuracy Badge -->
			<div class="hidden xl:flex items-center gap-1.5 px-3 py-1.5 rounded-lg bg-emerald-950/30 border border-emerald-800/40 text-emerald-300 text-[11px]">
				<Activity class="w-3.5 h-3.5 text-emerald-400" />
				<span>Benchmark:</span>
				<span class="font-bold">97.3% Verified</span>
			</div>
		</div>

		<!-- Right: Quick Keyboard Shortcuts Strip -->
		<div class="hidden lg:flex items-center gap-2 text-[11px] font-mono text-slate-400">
			<span class="text-slate-500">Shortcuts:</span>
			<span class="px-1.5 py-0.5 rounded bg-white/[0.05] border border-white/10 text-slate-300">J/K</span>
			<span class="text-slate-500">select</span>
			<span class="px-1.5 py-0.5 rounded bg-white/[0.05] border border-white/10 text-slate-300">R</span>
			<span class="text-slate-500">re-run</span>
			<span class="px-1.5 py-0.5 rounded bg-white/[0.05] border border-white/10 text-slate-300">A</span>
			<span class="text-slate-500">approve</span>
		</div>
	</div>
</div>
