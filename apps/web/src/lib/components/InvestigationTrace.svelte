<script lang="ts">
	import type { Run, RunStep } from '../api/types';

	interface Props {
		run: Run | null;
		steps: RunStep[];
		isLiveStreaming: boolean;
		onTriggerRun: () => void;
		isTriggering: boolean;
	}

	let {
		run = null,
		steps = [],
		isLiveStreaming = false,
		onTriggerRun,
		isTriggering = false
	}: Props = $props();

	let expandedSteps = $state<Record<string, boolean>>({});

	function toggleStep(id: string) {
		expandedSteps[id] = !expandedSteps[id];
	}

	function formatCost(cost: number | undefined): string {
		if (cost === undefined || cost === null) return '$0.0000';
		return `$${cost.toFixed(4)}`;
	}
</script>

<div class="space-y-4">
	<!-- Run Overview Header Bar -->
	{#if run}
		<div class="rounded-xl border border-neutral-800 bg-neutral-900/60 p-4 backdrop-blur-sm flex flex-wrap items-center justify-between gap-4">
			<div class="flex items-center gap-3">
				<div class="w-8 h-8 rounded-lg bg-indigo-950/80 border border-indigo-700/60 flex items-center justify-center text-indigo-400 font-mono font-bold text-xs">
					RUN
				</div>
				<div>
					<div class="flex items-center gap-2">
						<span class="text-xs font-mono text-neutral-300 font-bold">
							Run #{run.id.slice(0, 8)}
						</span>
						<!-- Run Status Badge -->
						<span class="px-2 py-0.5 rounded text-[10px] font-mono uppercase font-bold {
							run.status === 'succeeded'
								? 'bg-emerald-950 border border-emerald-800/80 text-emerald-300'
								: run.status === 'running'
									? 'bg-indigo-950 border border-indigo-800/80 text-indigo-300 animate-pulse'
									: 'bg-amber-950 border border-amber-800/80 text-amber-300'
						}">
							{run.status}
						</span>
					</div>
					<p class="text-[11px] text-neutral-400 font-mono">
						Started {new Date(run.started_at).toLocaleTimeString()}
					</p>
				</div>
			</div>

			<!-- Telemetry Badges (Tokens, Cost, Steps) -->
			<div class="flex items-center gap-4 text-xs font-mono">
				<div class="text-right">
					<span class="text-[10px] text-neutral-500 uppercase block">Tokens</span>
					<span class="text-neutral-200 font-semibold">{run.total_tokens?.toLocaleString() || '0'}</span>
				</div>
				<div class="text-right">
					<span class="text-[10px] text-neutral-500 uppercase block">Cost</span>
					<span class="text-emerald-400 font-semibold">{formatCost(run.total_cost_usd)}</span>
				</div>
				<div class="text-right">
					<span class="text-[10px] text-neutral-500 uppercase block">Steps</span>
					<span class="text-white font-semibold">{steps.length}</span>
				</div>

				<!-- Live Streaming SSE Indicator -->
				<div class="flex items-center gap-1.5 px-2.5 py-1 rounded bg-neutral-950 border border-neutral-800 text-[11px]">
					{#if isLiveStreaming}
						<span class="relative flex h-2 w-2">
							<span class="animate-ping absolute inline-flex h-full w-full rounded-full bg-indigo-400 opacity-75"></span>
							<span class="relative inline-flex rounded-full h-2 w-2 bg-indigo-500"></span>
						</span>
						<span class="text-indigo-400 font-medium">SSE Live</span>
					{:else}
						<span class="w-2 h-2 rounded-full bg-neutral-600 inline-block"></span>
						<span class="text-neutral-500">SSE Idle</span>
					{/if}
				</div>

				<button
					onclick={onTriggerRun}
					disabled={isTriggering}
					class="px-3 py-1.5 rounded-lg bg-indigo-600 hover:bg-indigo-500 text-white font-medium text-xs transition cursor-pointer disabled:opacity-50 flex items-center gap-1.5 shadow-sm active:scale-95"
				>
					<span>{isTriggering ? 'Running...' : 'Re-investigate'}</span>
				</button>
			</div>
		</div>
	{:else}
		<div class="rounded-xl border border-neutral-800 bg-neutral-900/40 p-6 text-center space-y-3">
			<p class="text-neutral-400 text-xs font-mono">No autonomous investigation runs found for this incident</p>
			<button
				onclick={onTriggerRun}
				disabled={isTriggering}
				class="px-4 py-2 rounded-lg bg-indigo-600 hover:bg-indigo-500 text-white font-medium text-xs transition cursor-pointer disabled:opacity-50 inline-flex items-center gap-2"
			>
				<span>🚀</span>
				<span>{isTriggering ? 'Dispatching Agent...' : 'Launch Autonomous Investigation'}</span>
			</button>
		</div>
	{/if}

	<!-- Timeline Step Trace -->
	<div class="space-y-3">
		{#each steps as step (step.id || step.step_number)}
			{@const isExpanded = expandedSteps[step.id || String(step.step_number)] ?? true}
			<div class="rounded-xl border border-neutral-800 bg-neutral-950/80 overflow-hidden transition">
				<!-- Step Header -->
				<button
					type="button"
					onclick={() => toggleStep(step.id || String(step.step_number))}
					class="w-full px-4 py-3 bg-neutral-900/60 hover:bg-neutral-900/90 flex items-center justify-between text-left text-xs transition border-b border-neutral-800/60 cursor-pointer"
				>
					<div class="flex items-center gap-3">
						<!-- Step Number Circle -->
						<div class="w-6 h-6 rounded-full bg-neutral-800 border border-neutral-700 flex items-center justify-center font-mono text-[11px] font-bold text-neutral-300">
							{step.step_number}
						</div>

						<!-- Step Type Badge -->
						{#if step.step_type === 'plan'}
							<span class="px-2 py-0.5 rounded bg-blue-950/80 border border-blue-800/80 text-blue-300 font-mono font-medium text-[10px]">
								PLAN
							</span>
							<span class="text-neutral-200 font-medium font-sans">Investigation Strategy & Hypotheses Plan</span>
						{:else if step.step_type === 'tool_call'}
							<span class="px-2 py-0.5 rounded bg-purple-950/80 border border-purple-800/80 text-purple-300 font-mono font-medium text-[10px]">
								TOOL CALL
							</span>
							<span class="font-mono text-purple-200 font-bold">{step.tool_name}</span>
						{:else if step.step_type === 'conclusion'}
							<span class="px-2 py-0.5 rounded bg-emerald-950/80 border border-emerald-800/80 text-emerald-300 font-mono font-medium text-[10px]">
								CONCLUSION
							</span>
							<span class="text-emerald-200 font-medium font-sans">Root-Cause Synthesis & Ranked Hypotheses</span>
						{:else}
							<span class="px-2 py-0.5 rounded bg-neutral-800 font-mono text-neutral-400 text-[10px]">
								{step.step_type}
							</span>
						{/if}
					</div>

					<!-- Latency & Toggle Arrow -->
					<div class="flex items-center gap-3 font-mono text-[11px] text-neutral-400">
						{#if step.latency_ms}
							<span class="bg-neutral-950 px-2 py-0.5 rounded border border-neutral-800 text-[10px]">
								{step.latency_ms}ms
							</span>
						{/if}
						<span class="text-neutral-500">{isExpanded ? '▲' : '▼'}</span>
					</div>
				</button>

				<!-- Step Body Content -->
				{#if isExpanded}
					<div class="p-4 space-y-3 text-xs font-mono">
						{#if step.step_type === 'plan' || step.step_type === 'conclusion'}
							<div class="p-3.5 rounded-lg bg-neutral-900/40 border border-neutral-800/80 text-neutral-300 font-sans leading-relaxed whitespace-pre-wrap">
								{typeof step.tool_output === 'object' && step.tool_output?.content
									? step.tool_output.content
									: typeof step.tool_output === 'string'
										? step.tool_output
										: JSON.stringify(step.tool_output, null, 2)}
							</div>
						{:else if step.step_type === 'tool_call'}
							<!-- Tool Input Args -->
							{#if step.tool_input}
								<div class="space-y-1">
									<span class="text-[10px] text-neutral-500 uppercase font-semibold">Tool Arguments:</span>
									<pre class="p-2.5 rounded-md bg-neutral-900 border border-neutral-800 text-[11px] text-indigo-300 overflow-x-auto">{JSON.stringify(step.tool_input, null, 2)}</pre>
								</div>
							{/if}

							<!-- Tool Output Observation -->
							{#if step.tool_output}
								<div class="space-y-1">
									<span class="text-[10px] text-neutral-500 uppercase font-semibold">Diagnostic Observation Payload:</span>
									<pre class="p-2.5 rounded-md bg-neutral-900 border border-neutral-800 text-[11px] text-emerald-300 overflow-x-auto max-h-56">{JSON.stringify(step.tool_output, null, 2)}</pre>
								</div>
							{/if}
						{/if}
					</div>
				{/if}
			</div>
		{/each}
	</div>
</div>
