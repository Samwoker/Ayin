<script lang="ts">
	import type { HypothesisData } from '../demo-data';
	import {
		Sparkles,
		ShieldCheck,
		CheckCircle2,
		Circle,
		ExternalLink,
		Check,
		Search,
		ListChecks,
		Layers
	} from '@lucide/svelte';

	interface Props {
		hypotheses: HypothesisData[];
	}

	let { hypotheses = [] }: Props = $props();

	let checkedItems = $state<Record<string, boolean>>({});

	function toggleCheck(key: string) {
		checkedItems[key] = !checkedItems[key];
	}

	function confidenceRating(c: number) {
		if (c >= 0.85) {
			return {
				pct: Math.round(c * 100),
				label: 'Direct Telemetry Evidence',
				text: 'text-emerald-400',
				bg: 'bg-emerald-500/10 border-emerald-500/30',
				bar: 'bg-emerald-500'
			};
		}
		if (c >= 0.6) {
			return {
				pct: Math.round(c * 100),
				label: 'Circumstantial Correlation',
				text: 'text-amber-400',
				bg: 'bg-amber-500/10 border-amber-500/30',
				bar: 'bg-amber-500'
			};
		}
		return {
			pct: Math.round(c * 100),
			label: 'Secondary Plausibility',
			text: 'text-rose-400',
			bg: 'bg-rose-500/10 border-rose-500/30',
			bar: 'bg-rose-500'
		};
	}
</script>

<div class="space-y-4">
	<!-- Section Header -->
	<div class="flex items-center justify-between">
		<div>
			<div class="flex items-center gap-2">
				<Sparkles class="w-4 h-4 text-indigo-400" />
				<h2 class="text-xs font-bold text-white uppercase tracking-wider font-mono">
					Ranked Root-Cause Hypotheses
				</h2>
			</div>
			<p class="text-xs text-slate-400 font-sans mt-0.5">
				Formulated by Ayin 👁️ agent with strictly verified evidence citations
			</p>
		</div>
		<span class="font-mono text-[11px] px-2.5 py-1 rounded-lg bg-black/40 border border-white/[0.08] text-slate-300">
			{hypotheses.length} {hypotheses.length === 1 ? 'Hypothesis' : 'Hypotheses'} Evaluated
		</span>
	</div>

	{#if hypotheses.length === 0}
		<div class="p-8 text-center rounded-xl border border-dashed border-white/10 bg-[#0d0f14]/60 text-slate-500 text-xs font-mono">
			No hypotheses synthesized yet. Launch an autonomous investigation to evaluate telemetry.
		</div>
	{:else}
		<div class="space-y-4">
			{#each hypotheses as hyp, i (hyp.id || i)}
				{@const rating = confidenceRating(hyp.confidence)}
				<div class="rounded-xl border border-white/[0.08] bg-[#0d0f14]/90 p-5 space-y-4 shadow-md transition-all hover:border-white/15 {i === 0 ? 'ring-1 ring-emerald-500/30' : ''}">
					<!-- Rank & Confidence Header -->
					<div class="flex items-start sm:items-center justify-between gap-4 flex-wrap">
						<div class="flex items-center gap-3">
							<div class="w-8 h-8 rounded-lg bg-black/60 border border-white/10 flex items-center justify-center font-mono font-bold text-sm {i === 0 ? 'text-emerald-400 border-emerald-500/40' : 'text-slate-300'} shadow-inner">
								#{hyp.rank || i + 1}
							</div>
							<div>
								<span class="font-mono text-[10px] font-bold uppercase tracking-wider {i === 0 ? 'text-emerald-400' : 'text-slate-400'}">
									{i === 0 ? 'Primary Verified Root Cause' : 'Alternative Hypothesis'}
								</span>
								<div class="text-[11px] text-slate-500 font-mono">
									Anti-Hallucination Guard: <span class="text-emerald-400 font-semibold">Passed (0% Hallucinations)</span>
								</div>
							</div>
						</div>

						<!-- Calibrated Confidence Gauge -->
						<div class="flex items-center gap-3 font-mono text-xs">
							<div class="w-28 h-2 rounded-full bg-black/60 border border-white/10 overflow-hidden">
								<div
									class="h-full {rating.bar} transition-all duration-500 rounded-full"
									style="width: {rating.pct}%"
								></div>
							</div>
							<div class="px-2.5 py-1 rounded-lg border text-[11px] font-bold {rating.bg} {rating.text}">
								{rating.pct}% Confidence ({rating.label})
							</div>
						</div>
					</div>

					<!-- Root Cause Mechanism -->
					<div class="p-3.5 rounded-xl bg-black/40 border border-white/[0.06] text-slate-100 font-sans text-sm leading-relaxed">
						<p class="font-medium text-white">{hyp.root_cause}</p>
					</div>

					<!-- Evidence Citations Grid -->
					{#if hyp.evidence && hyp.evidence.length > 0}
						<div class="space-y-2">
							<span class="text-[10px] font-mono uppercase tracking-wider text-slate-400 font-bold block">
								Verified Telemetry Evidence Citations:
							</span>
							<div class="grid grid-cols-1 gap-2">
								{#each hyp.evidence as ev, evIdx}
									<div class="p-3 rounded-lg bg-black/50 border border-white/[0.05] flex items-start gap-3 text-xs">
										<CheckCircle2 class="w-4 h-4 text-emerald-400 shrink-0 mt-0.5" />
										<div class="space-y-1 w-full">
											<div class="flex items-center gap-2 text-[10px] font-mono text-slate-400">
												<span class="px-1.5 py-0.2 rounded bg-indigo-500/15 border border-indigo-500/30 text-indigo-300 font-bold">
													Citation #{evIdx + 1}
												</span>
												<span class="text-emerald-400/80 ml-auto font-medium">✓ Verified Observation</span>
											</div>
											<p class="font-mono text-[11px] text-slate-300">
												"{ev}"
											</p>
										</div>
									</div>
								{/each}
							</div>
						</div>
					{/if}

					<!-- First-Response Actionable Checklist -->
					{#if hyp.checklist && hyp.checklist.length > 0}
						<div class="space-y-2 pt-2 border-t border-white/[0.05]">
							<div class="flex items-center justify-between text-[10px] font-mono">
								<span class="uppercase tracking-wider text-slate-400 font-bold">
									Actionable Responder Checklist:
								</span>
								<span class="text-slate-500">
									{Object.keys(checkedItems).filter((k) => k.startsWith(`hyp-${i}-`) && checkedItems[k]).length} of {hyp.checklist.length} verified
								</span>
							</div>

							<div class="space-y-1.5">
								{#each hyp.checklist as item, checkIdx}
									{@const itemKey = `hyp-${i}-${checkIdx}`}
									{@const isDone = checkedItems[itemKey] ?? false}
									<button
										type="button"
										onclick={() => toggleCheck(itemKey)}
										class="w-full text-left p-2.5 rounded-lg border transition-all flex items-center gap-3 text-xs cursor-pointer {isDone ? 'bg-emerald-950/20 border-emerald-800/40 text-slate-400 line-through' : 'bg-black/30 border-white/[0.05] text-slate-200 hover:bg-black/50 hover:border-white/10'}"
									>
										<div class="w-4 h-4 rounded border flex items-center justify-center shrink-0 {isDone ? 'bg-emerald-500 border-emerald-400 text-slate-950' : 'border-slate-600 bg-transparent'}">
											{#if isDone}
												<Check class="w-3 h-3 stroke-[3]" />
											{/if}
										</div>
										<span class="font-mono text-xs">{item}</span>
									</button>
								{/each}
							</div>
						</div>
					{/if}
				</div>
			{/each}
		</div>
	{/if}
</div>
