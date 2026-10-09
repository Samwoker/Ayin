<script lang="ts">
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
	<div class="rounded-xl border border-neutral-800 bg-neutral-900/40 p-4 backdrop-blur-sm flex items-center justify-between">
		<div>
			<span class="text-[11px] font-mono uppercase tracking-wider text-neutral-400 font-semibold">Active Incidents</span>
			<div class="text-2xl font-bold text-white mt-1">{activeCount}</div>
		</div>
		<div class="w-10 h-10 rounded-lg bg-rose-950/60 border border-rose-800/40 flex items-center justify-center text-rose-400 text-lg">
			🚨
		</div>
	</div>

	<!-- Critical / Sev1 -->
	<div class="rounded-xl border border-neutral-800 bg-neutral-900/40 p-4 backdrop-blur-sm flex items-center justify-between">
		<div>
			<span class="text-[11px] font-mono uppercase tracking-wider text-rose-400 font-semibold">Sev1 / Critical</span>
			<div class="text-2xl font-bold text-rose-400 mt-1">{sev1Count}</div>
		</div>
		<div class="w-10 h-10 rounded-lg bg-red-950/80 border border-red-800/60 flex items-center justify-center text-red-300 font-mono font-bold text-sm">
			SEV1
		</div>
	</div>

	<!-- Autonomous Runs -->
	<div class="rounded-xl border border-neutral-800 bg-neutral-900/40 p-4 backdrop-blur-sm flex items-center justify-between">
		<div>
			<span class="text-[11px] font-mono uppercase tracking-wider text-neutral-400 font-semibold">Autonomous Runs</span>
			<div class="text-2xl font-bold text-indigo-400 mt-1">{runsCount}</div>
		</div>
		<div class="w-10 h-10 rounded-lg bg-indigo-950/60 border border-indigo-800/40 flex items-center justify-center text-indigo-400 text-lg">
			🤖
		</div>
	</div>

	<!-- Pending Remediations (HITL Approval Required) -->
	<div class="rounded-xl border border-neutral-800 bg-neutral-900/40 p-4 backdrop-blur-sm flex items-center justify-between">
		<div>
			<span class="text-[11px] font-mono uppercase tracking-wider text-amber-400 font-semibold">HITL Approvals</span>
			<div class="text-2xl font-bold text-amber-400 mt-1">{pendingRemediationsCount}</div>
		</div>
		<div class="w-10 h-10 rounded-lg bg-amber-950/60 border border-amber-800/40 flex items-center justify-center text-amber-400 text-lg">
			🛡️
		</div>
	</div>
</div>
