<script lang="ts">
	import type { Run, RunStep } from '../api/types';
	import { Button } from '#lib/components/ui/button/index.js';
	import { Badge } from '#lib/components/ui/badge/index.js';
	import { Card } from '#lib/components/ui/card/index.js';

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
		<Card class="p-4 border-slate-800 bg-slate-900/60 backdrop-blur-sm flex flex-wrap items-center justify-between gap-4">
			<div class="flex items-center gap-3">
				<div class="w-8 h-8 rounded-lg bg-indigo-950/80 border border-indigo-700/60 flex items-center justify-center text-indigo-400 font-mono font-bold text-xs">
					RUN
				</div>
				<div>
					<div class="flex items-center gap-2">
						<span class="text-xs font-mono text-slate-300 font-bold">
							Run #{run.id.slice(0, 8)}
						</span>
						<!-- Run Status Badge -->
						<Badge
							variant={run.status === 'succeeded'
								? 'default'
								: run.status === 'running'
									? 'sev3'
									: 'warning'}
							class="uppercase font-mono text-[10px]"
						>
							{run.status}
						</Badge>
					</div>
					<p class="text-[11px] text-slate-400 font-mono">
						Started {new Date(run.started_at).toLocaleTimeString()}
					</p>
				</div>
			</div>

			<!-- Telemetry Badges (Tokens, Cost, Steps) -->
			<div class="flex items-center gap-4 text-xs font-mono">
				<div class="text-right">
					<span class="text-[10px] text-slate-500 uppercase block">Tokens</span>
					<span class="text-slate-200 font-semibold">{run.total_tokens?.toLocaleString() || '0'}</span>
				</div>
				<div class="text-right">
					<span class="text-[10px] text-slate-500 uppercase block">Cost</span>
					<span class="text-emerald-400 font-semibold">{formatCost(run.total_cost_usd)}</span>
				</div>
				<div class="text-right">
					<span class="text-[10px] text-slate-500 uppercase block">Steps</span>
					<span class="text-white font-semibold">{steps.length}</span>
				</div>

				<!-- Live Streaming SSE Indicator -->
				<div class="flex items-center gap-1.5 px-2.5 py-1 rounded bg-slate-950 border border-slate-800 text-[11px]">
					{#if isLiveStreaming}
						<span class="relative flex h-2 w-2">
							<span class="animate-ping absolute inline-flex h-full w-full rounded-full bg-indigo-400 opacity-75"></span>
							<span class="relative inline-flex rounded-full h-2 w-2 bg-indigo-500"></span>
						</span>
						<span class="text-indigo-400 font-medium">SSE Live</span>
					{:else}
						<span class="w-2 h-2 rounded-full bg-slate-600 inline-block"></span>
						<span class="text-slate-500">SSE Idle</span>
					{/if}
				</div>

				<Button
					size="sm"
					onclick={onTriggerRun}
					disabled={isTriggering}
					class="bg-indigo-600 hover:bg-indigo-500 text-white gap-1.5"
				>
					<span>{isTriggering ? 'Running...' : 'Re-investigate'}</span>
				</Button>
			</div>
		</Card>
	{:else}
		<Card class="border-slate-800 bg-slate-900/40 p-6 text-center space-y-3">
			<p class="text-slate-400 text-xs font-mono">No autonomous investigation runs found for this incident</p>
			<Button
				onclick={onTriggerRun}
				disabled={isTriggering}
				class="bg-indigo-600 hover:bg-indigo-500 text-white gap-2"
			>
				<span>🚀</span>
				<span>{isTriggering ? 'Dispatching Agent...' : 'Launch Autonomous Investigation'}</span>
			</Button>
		</Card>
	{/if}

	<!-- Timeline Step Trace -->
	<div class="space-y-3">
		{#each steps as step (step.id || step.step_number)}
			{@const isExpanded = expandedSteps[step.id || String(step.step_number)] ?? true}
			<Card class="border-slate-800 bg-slate-950/80 overflow-hidden transition p-0">
				<!-- Step Header -->
				<button
					type="button"
					onclick={() => toggleStep(step.id || String(step.step_number))}
					class="w-full px-4 py-3 bg-slate-900/60 hover:bg-slate-900/90 flex items-center justify-between text-left text-xs transition border-b border-slate-800/60 cursor-pointer"
				>
					<div class="flex items-center gap-3">
						<!-- Step Number Circle -->
						<div class="w-6 h-6 rounded-full bg-slate-800 border border-slate-700 flex items-center justify-center font-mono text-[11px] font-bold text-slate-300">
							{step.step_number}
						</div>

						<!-- Step Type Badge -->
						{#if step.step_type === 'plan'}
							<Badge variant="sev3" class="font-mono text-[10px]">
								PLAN
							</Badge>
							<span class="text-slate-200 font-medium font-sans">Investigation Strategy & Hypotheses Plan</span>
						{:else if step.step_type === 'tool_call'}
							<Badge variant="secondary" class="bg-purple-950/80 border-purple-800/80 text-purple-300 font-mono text-[10px]">
								TOOL CALL
							</Badge>
							<span class="font-mono text-purple-200 font-bold">{step.tool_name}</span>
						{:else if step.step_type === 'conclusion'}
							<Badge variant="default" class="font-mono text-[10px]">
								CONCLUSION
							</Badge>
							<span class="text-emerald-200 font-medium font-sans">Root-Cause Synthesis & Ranked Hypotheses</span>
						{:else}
							<Badge variant="outline" class="font-mono text-[10px]">
								{step.step_type}
							</Badge>
						{/if}
					</div>

					<!-- Latency & Toggle Arrow -->
					<div class="flex items-center gap-3 font-mono text-[11px] text-slate-400">
						{#if step.latency_ms}
							<span class="bg-slate-950 px-2 py-0.5 rounded border border-slate-800 text-[10px]">
								{step.latency_ms}ms
							</span>
						{/if}
						<span class="text-slate-500">{isExpanded ? '▲' : '▼'}</span>
					</div>
				</button>

				<!-- Step Body Content -->
				{#if isExpanded}
					<div class="p-4 space-y-3 text-xs font-mono">
						{#if step.step_type === 'plan' || step.step_type === 'conclusion'}
							<div class="p-3.5 rounded-lg bg-slate-900/40 border border-slate-800/80 text-slate-300 font-sans leading-relaxed whitespace-pre-wrap">
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
									<span class="text-[10px] text-slate-500 uppercase font-semibold">Tool Arguments:</span>
									<pre class="p-2.5 rounded-md bg-slate-900 border border-slate-800 text-[11px] text-indigo-300 overflow-x-auto">{JSON.stringify(step.tool_input, null, 2)}</pre>
								</div>
							{/if}

							<!-- Tool Output Observation -->
							{#if step.tool_output}
								<div class="space-y-1">
									<span class="text-[10px] text-slate-500 uppercase font-semibold">Diagnostic Observation Payload:</span>
									<pre class="p-2.5 rounded-md bg-slate-900 border border-slate-800 text-[11px] text-emerald-300 overflow-x-auto max-h-56">{JSON.stringify(step.tool_output, null, 2)}</pre>
								</div>
							{/if}
						{/if}
					</div>
				{/if}
			</Card>
		{/each}
	</div>
</div>
