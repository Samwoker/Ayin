<script lang="ts">
	import type { Run, RunStep } from '../api/types';
	import { Button } from '#lib/components/ui/button/index.js';
	import {
		Bot,
		Terminal,
		Play,
		Coins,
		DollarSign,
		Layers,
		ChevronDown,
		ChevronUp,
		Cpu,
		Clock,
		CheckCircle2,
		Radio,
		Copy,
		Check,
		Search,
		Filter
	} from '@lucide/svelte';

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
	let copiedStepId = $state<string | null>(null);
	let stepFilter = $state<'all' | 'tools' | 'plans'>('all');

	function toggleStep(id: string) {
		expandedSteps[id] = !(expandedSteps[id] ?? true);
	}

	function copyPayload(stepId: string, text: string) {
		navigator.clipboard.writeText(text);
		copiedStepId = stepId;
		setTimeout(() => {
			if (copiedStepId === stepId) copiedStepId = null;
		}, 2000);
	}

	function formatCost(cost: number | undefined): string {
		if (cost === undefined || cost === null) return '$0.0000';
		return `$${cost.toFixed(4)}`;
	}

	let filteredSteps = $derived(
		steps.filter((s) => {
			if (stepFilter === 'tools') return s.step_type === 'tool_call';
			if (stepFilter === 'plans') return s.step_type === 'plan' || s.step_type === 'reasoning';
			return true;
		})
	);
</script>

<div class="space-y-4">
	<!-- Run Overview Header Bar -->
	{#if run}
		<div class="rounded-xl border border-white/[0.08] bg-[#0d0f14]/90 backdrop-blur-md p-4 flex flex-wrap items-center justify-between gap-4 shadow-md">
			<div class="flex items-center gap-3">
				<div class="w-9 h-9 rounded-lg bg-indigo-500/10 border border-indigo-500/30 flex items-center justify-center text-indigo-400 font-mono font-bold text-xs shadow-inner">
					<Cpu class="w-4 h-4 text-indigo-400" />
				</div>
				<div>
					<div class="flex items-center gap-2">
						<span class="text-xs font-mono text-white font-bold">
							Agent Run #{run.id.slice(0, 8)}
						</span>
						<span class="px-2 py-0.5 rounded-full font-mono text-[10px] font-bold uppercase {run.status === 'succeeded' ? 'bg-emerald-500/15 border border-emerald-500/30 text-emerald-300' : run.status === 'running' ? 'bg-indigo-500/20 border border-indigo-500/40 text-indigo-300 animate-pulse' : 'bg-amber-500/15 border border-amber-500/30 text-amber-300'}">
							{run.status}
						</span>
					</div>
					<p class="text-[11px] text-slate-400 font-mono flex items-center gap-1.5 mt-0.5">
						<Clock class="w-3 h-3 text-slate-500" />
						Started {new Date(run.started_at).toLocaleTimeString()}
					</p>
				</div>
			</div>

			<!-- Telemetry Badges (Tokens, Cost, Steps, SSE) -->
			<div class="flex items-center gap-2.5 text-xs font-mono flex-wrap">
				<div class="flex items-center gap-1.5 bg-black/40 px-2.5 py-1.5 rounded-lg border border-white/[0.06]">
					<Coins class="w-3.5 h-3.5 text-slate-400" />
					<span class="text-slate-400 text-[10px]">Tokens:</span>
					<span class="text-white font-bold">{run.total_tokens?.toLocaleString() || '0'}</span>
				</div>

				<div class="flex items-center gap-1.5 bg-black/40 px-2.5 py-1.5 rounded-lg border border-white/[0.06]">
					<DollarSign class="w-3.5 h-3.5 text-emerald-400" />
					<span class="text-slate-400 text-[10px]">Cost:</span>
					<span class="text-emerald-400 font-bold">{formatCost(run.total_cost_usd)}</span>
				</div>

				<div class="flex items-center gap-1.5 bg-black/40 px-2.5 py-1.5 rounded-lg border border-white/[0.06]">
					<Layers class="w-3.5 h-3.5 text-indigo-400" />
					<span class="text-slate-400 text-[10px]">Steps:</span>
					<span class="text-indigo-300 font-bold">{steps.length}</span>
				</div>

				<!-- Live Streaming SSE Indicator -->
				<div class="flex items-center gap-1.5 px-2.5 py-1.5 rounded-lg bg-black/50 border border-white/[0.06] text-[11px]">
					{#if isLiveStreaming}
						<span class="relative flex h-2 w-2">
							<span class="animate-ping absolute inline-flex h-full w-full rounded-full bg-emerald-400 opacity-75"></span>
							<span class="relative inline-flex rounded-full h-2 w-2 bg-emerald-500"></span>
						</span>
						<span class="text-emerald-400 font-medium">SSE Stream Active</span>
					{:else}
						<span class="w-1.5 h-1.5 rounded-full bg-slate-600"></span>
						<span class="text-slate-500">Stream Idle</span>
					{/if}
				</div>

				<Button
					size="sm"
					onclick={onTriggerRun}
					disabled={isTriggering}
					class="bg-indigo-600 hover:bg-indigo-500 text-white font-medium text-xs h-8 px-3 gap-1.5 shadow-sm active:scale-95 transition"
				>
					<Play class="w-3 h-3 fill-current" />
					<span>{isTriggering ? 'Running...' : 'Re-investigate'}</span>
				</Button>
			</div>
		</div>
	{:else}
		<div class="rounded-xl border border-dashed border-white/10 bg-[#0d0f14]/60 p-8 text-center space-y-3">
			<Bot class="w-8 h-8 text-indigo-400 mx-auto" />
			<p class="text-slate-400 text-xs font-mono">No autonomous investigation runs found for this incident</p>
			<Button
				onclick={onTriggerRun}
				disabled={isTriggering}
				class="bg-indigo-600 hover:bg-indigo-500 text-white font-medium text-xs h-8.5 px-4 gap-2 shadow-sm active:scale-95 transition"
			>
				<Play class="w-3.5 h-3.5 fill-current" />
				<span>Launch Autonomous Investigation</span>
			</Button>
		</div>
	{/if}

	<!-- Trace Filter Bar -->
	{#if steps.length > 0}
		<div class="flex items-center justify-between text-xs font-mono px-1">
			<div class="flex items-center gap-1.5">
				<Filter class="w-3.5 h-3.5 text-slate-500" />
				<span class="text-slate-500 text-[11px]">Filter Trace:</span>
				<button
					onclick={() => (stepFilter = 'all')}
					class="px-2 py-0.5 rounded text-[11px] transition {stepFilter === 'all' ? 'bg-white/10 text-white font-bold' : 'text-slate-400 hover:text-white'}"
				>
					All ({steps.length})
				</button>
				<button
					onclick={() => (stepFilter = 'tools')}
					class="px-2 py-0.5 rounded text-[11px] transition {stepFilter === 'tools' ? 'bg-indigo-500/20 text-indigo-300 font-bold' : 'text-slate-400 hover:text-white'}"
				>
					Tools Only ({steps.filter((s) => s.step_type === 'tool_call').length})
				</button>
				<button
					onclick={() => (stepFilter = 'plans')}
					class="px-2 py-0.5 rounded text-[11px] transition {stepFilter === 'plans' ? 'bg-amber-500/20 text-amber-300 font-bold' : 'text-slate-400 hover:text-white'}"
				>
					Plans Only ({steps.filter((s) => s.step_type === 'plan').length})
				</button>
			</div>

			<span class="text-[11px] text-slate-500">
				Deterministic timeline • Read-only tools
			</span>
		</div>
	{/if}

	<!-- Timeline Step Trace (Threaded) -->
	<div class="space-y-3 relative">
		{#each filteredSteps as step, idx (step.id || step.step_number)}
			{@const stepKey = step.id || String(step.step_number)}
			{@const isExpanded = expandedSteps[stepKey] ?? true}
			{@const isTool = step.step_type === 'tool_call'}

			<div class="rounded-xl border border-white/[0.07] bg-[#0d0f14]/90 overflow-hidden shadow-sm transition-all hover:border-white/15">
				<!-- Step Trigger Header -->
				<button
					type="button"
					onclick={() => toggleStep(stepKey)}
					class="w-full text-left p-3.5 flex items-center justify-between gap-4 cursor-pointer hover:bg-white/[0.02] transition"
				>
					<div class="flex items-center gap-3">
						<!-- Step Number Badge -->
						<div class="w-7 h-7 rounded-lg font-mono font-bold text-xs flex items-center justify-center {isTool ? 'bg-indigo-500/10 border border-indigo-500/30 text-indigo-300' : 'bg-amber-500/10 border border-amber-500/30 text-amber-300'}">
							{step.step_number || idx + 1}
						</div>

						<div>
							<div class="flex items-center gap-2 flex-wrap">
								<!-- Step Type -->
								<span class="font-mono text-[10px] font-bold px-2 py-0.5 rounded uppercase {isTool ? 'bg-indigo-500/15 text-indigo-300 border border-indigo-500/30' : 'bg-amber-500/15 text-amber-300 border border-amber-500/30'}">
									{step.step_type === 'tool_call' ? 'Tool Call' : 'Plan / Strategy'}
								</span>

								<!-- Tool Name / Title -->
								<span class="font-mono text-xs font-bold text-white">
									{step.tool_name || 'Investigation Strategy & Hypotheses Formulation'}
								</span>

								<!-- Latency Badge -->
								{#if step.latency_ms}
									<span class="text-[10px] font-mono text-slate-400 bg-black/40 px-1.5 py-0.5 rounded border border-white/[0.05]">
										{step.latency_ms}ms
									</span>
								{/if}
							</div>
						</div>
					</div>

					<!-- Chevron -->
					<div class="text-slate-500 hover:text-slate-300">
						{#if isExpanded}
							<ChevronUp class="w-4 h-4" />
						{:else}
							<ChevronDown class="w-4 h-4" />
						{/if}
					</div>
				</button>

				<!-- Expanded Content -->
				{#if isExpanded}
					<div class="px-4 pb-4 pt-1 space-y-3 border-t border-white/[0.05] bg-black/30 text-xs">
						<!-- Strategy / Plan Text -->
						{#if !isTool && step.tool_input}
							<div class="p-3.5 rounded-lg bg-black/40 border border-white/[0.05] text-slate-300 font-sans text-xs leading-relaxed space-y-2">
								<p class="text-slate-200">
									{typeof step.tool_input === 'string' ? step.tool_input : JSON.stringify(step.tool_input, null, 2)}
								</p>
							</div>
						{/if}

						<!-- Tool Arguments -->
						{#if isTool && step.tool_input}
							<div class="space-y-1.5">
								<div class="flex items-center justify-between text-[11px] font-mono text-slate-400">
									<span class="uppercase tracking-wider text-[10px] text-slate-500">Tool Parameters:</span>
									<button
										onclick={() => copyPayload(stepKey + '-in', typeof step.tool_input === 'string' ? step.tool_input : JSON.stringify(step.tool_input, null, 2))}
										class="flex items-center gap-1 text-slate-400 hover:text-white transition"
									>
										{#if copiedStepId === stepKey + '-in'}
											<Check class="w-3 h-3 text-emerald-400" />
											<span class="text-emerald-400">Copied</span>
										{:else}
											<Copy class="w-3 h-3" />
											<span>Copy</span>
										{/if}
									</button>
								</div>
								<pre class="p-3 rounded-lg bg-[#08090c] border border-white/[0.06] font-mono text-[11px] text-indigo-300 overflow-x-auto leading-relaxed">{typeof step.tool_input === 'string' ? step.tool_input : JSON.stringify(step.tool_input, null, 2)}</pre>
							</div>
						{/if}

						<!-- Tool Output / Diagnostic Observations -->
						{#if isTool && step.tool_output}
							<div class="space-y-1.5">
								<div class="flex items-center justify-between text-[11px] font-mono text-slate-400">
									<span class="uppercase tracking-wider text-[10px] text-slate-500">Diagnostic Observation Payload:</span>
									<button
										onclick={() => copyPayload(stepKey + '-out', typeof step.tool_output === 'string' ? step.tool_output : JSON.stringify(step.tool_output, null, 2))}
										class="flex items-center gap-1 text-slate-400 hover:text-white transition"
									>
										{#if copiedStepId === stepKey + '-out'}
											<Check class="w-3 h-3 text-emerald-400" />
											<span class="text-emerald-400">Copied</span>
										{:else}
											<Copy class="w-3 h-3" />
											<span>Copy</span>
										{/if}
									</button>
								</div>
								<pre class="p-3 rounded-lg bg-[#08090c] border border-white/[0.06] font-mono text-[11px] text-emerald-300/90 overflow-x-auto leading-relaxed">{typeof step.tool_output === 'string' ? step.tool_output : JSON.stringify(step.tool_output, null, 2)}</pre>
							</div>
						{/if}
					</div>
				{/if}
			</div>
		{/each}
	</div>
</div>
