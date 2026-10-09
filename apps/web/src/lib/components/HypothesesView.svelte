<script lang="ts">
	import type { HypothesisData } from '../demo-data';
	import { Card } from '#lib/components/ui/card/index.js';
	import { Badge } from '#lib/components/ui/badge/index.js';
	import { Separator } from '#lib/components/ui/separator/index.js';

	interface Props {
		hypotheses: HypothesisData[];
	}

	let { hypotheses = [] }: Props = $props();

	let checkedItems = $state<Record<string, boolean>>({});

	function toggleCheck(key: string) {
		checkedItems[key] = !checkedItems[key];
	}

	function confidenceColor(c: number): { text: string; bg: string; border: string; bar: string } {
		if (c >= 0.85) {
			return {
				text: 'text-emerald-400',
				bg: 'bg-emerald-950/80',
				border: 'border-emerald-800/80',
				bar: 'bg-emerald-500'
			};
		}
		if (c >= 0.6) {
			return {
				text: 'text-amber-400',
				bg: 'bg-amber-950/80',
				border: 'border-amber-800/80',
				bar: 'bg-amber-500'
			};
		}
		return {
			text: 'text-rose-400',
			bg: 'bg-rose-950/80',
			border: 'border-rose-800/80',
			bar: 'bg-rose-500'
		};
	}
</script>

<div class="space-y-4">
	<div class="flex items-center justify-between">
		<div>
			<h2 class="text-sm font-bold text-white uppercase tracking-wider font-mono">
				Ranked Root-Cause Hypotheses
			</h2>
			<p class="text-xs text-slate-400">
				Evidence-substantiated causality models formulated by autonomous agent
			</p>
		</div>
		<Badge variant="secondary" class="font-mono text-[11px]">
			{hypotheses.length} {hypotheses.length === 1 ? 'Hypothesis' : 'Hypotheses'} Evaluated
		</Badge>
	</div>

	{#if hypotheses.length === 0}
		<div class="p-8 text-center rounded-xl border border-dashed border-slate-800 text-slate-500 text-xs font-mono">
			No hypotheses synthesized yet. Launch an autonomous investigation to analyze telemetry.
		</div>
	{:else}
		<div class="space-y-4">
			{#each hypotheses as hyp, i (hyp.id || i)}
				{@const colors = confidenceColor(hyp.confidence)}
				<Card class="p-5 space-y-4 border-slate-800 bg-slate-900/60 backdrop-blur-sm">
					<!-- Top Row: Rank & Confidence Meter -->
					<div class="flex items-start sm:items-center justify-between gap-4 flex-wrap">
						<div class="flex items-center gap-3">
							<div class="w-8 h-8 rounded-lg bg-slate-950 border border-slate-800 flex items-center justify-center font-mono font-bold text-sm text-white shadow-inner">
								#{hyp.rank || i + 1}
							</div>
							<span class="font-mono text-xs font-bold text-slate-300 uppercase tracking-wider">
								{i === 0 ? 'Primary Root Cause' : 'Secondary Hypothesis'}
							</span>
						</div>

						<!-- Confidence Gauge -->
						<div class="flex items-center gap-3 bg-slate-950 px-3 py-1.5 rounded-lg border border-slate-800">
							<div class="text-right">
								<span class="text-[10px] text-slate-500 uppercase font-mono block">Confidence</span>
								<span class="text-sm font-bold font-mono {colors.text}">
									{Math.round(hyp.confidence * 100)}%
								</span>
							</div>
							<div class="w-20 bg-slate-800 h-2 rounded-full overflow-hidden">
								<div class="h-full {colors.bar} transition-all duration-500" style="width: {hyp.confidence * 100}%"></div>
							</div>
						</div>
					</div>

					<!-- Root Cause Description -->
					<div class="p-3.5 rounded-lg bg-slate-950 border border-slate-800/80">
						<h3 class="text-sm font-medium text-white leading-relaxed font-sans">
							{hyp.root_cause}
						</h3>
					</div>

					<!-- Evidence Citations List -->
					<div class="space-y-2">
						<span class="text-[11px] font-mono font-bold uppercase tracking-wider text-slate-400 flex items-center gap-1.5">
							<span>🔎</span>
							<span>Observed Telemetry Evidence:</span>
						</span>
						<ul class="space-y-1.5 text-xs font-mono">
							{#each hyp.evidence as item}
								<li class="flex items-start gap-2.5 p-2 rounded bg-slate-950/60 border border-slate-800/60 text-slate-300">
									<span class="text-emerald-400 select-none">✓</span>
									<span class="leading-relaxed font-sans text-xs text-slate-200">{item}</span>
								</li>
							{/each}
						</ul>
					</div>

					<Separator class="bg-slate-800/60" />

					<!-- First-Response Checklist -->
					<div class="space-y-2 pt-1">
						<span class="text-[11px] font-mono font-bold uppercase tracking-wider text-indigo-400 flex items-center gap-1.5">
							<span>📋</span>
							<span>First-Response Action Checklist:</span>
						</span>
						<div class="space-y-1.5">
							{#each hyp.checklist as checkItem, ci}
								{@const checkKey = `${hyp.id || i}-${ci}`}
								{@const isChecked = checkedItems[checkKey] ?? false}
								<label
									class="flex items-center gap-3 p-2.5 rounded-lg border transition cursor-pointer text-xs {isChecked ? 'bg-emerald-950/20 border-emerald-900/40 text-slate-500 line-through' : 'bg-slate-950/80 border-slate-800 text-slate-200 hover:border-slate-700'}"
								>
									<input
										type="checkbox"
										checked={isChecked}
										onchange={() => toggleCheck(checkKey)}
										class="w-4 h-4 accent-emerald-500 rounded cursor-pointer"
									/>
									<span class="font-sans leading-normal">{checkItem}</span>
								</label>
							{/each}
						</div>
					</div>
				</Card>
			{/each}
		</div>
	{/if}
</div>
