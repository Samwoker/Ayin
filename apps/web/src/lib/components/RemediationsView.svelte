<script lang="ts">
	import type { Remediation, RemediationEvent } from '../api/types';

	interface Props {
		remediations: Remediation[];
		events: Record<string, RemediationEvent[]>;
		onApprove: (rem: Remediation) => void;
		onReject: (rem: Remediation) => void;
		onRollback: (rem: Remediation) => void;
		onProposeCustom: () => void;
	}

	let {
		remediations = [],
		events = {},
		onApprove,
		onReject,
		onRollback,
		onProposeCustom
	}: Props = $props();

	function getActionIcon(action: string): string {
		switch (action) {
			case 'restart_service':
				return '🔄';
			case 'scale_service':
				return '📈';
			case 'rollback_deploy':
				return '⏪';
			case 'clear_cache':
				return '🧹';
			case 'pause_consumer':
				return '⏸️';
			case 'toggle_feature_flag':
				return '🚩';
			default:
				return '⚡';
		}
	}

	function getRiskBadge(risk: string): { text: string; bg: string; border: string } {
		switch (risk) {
			case 'critical':
				return { text: 'text-red-400', bg: 'bg-red-950/80', border: 'border-red-800' };
			case 'high':
				return { text: 'text-orange-400', bg: 'bg-orange-950/80', border: 'border-orange-800' };
			case 'medium':
				return { text: 'text-amber-400', bg: 'bg-amber-950/80', border: 'border-amber-800' };
			default:
				return { text: 'text-emerald-400', bg: 'bg-emerald-950/80', border: 'border-emerald-800' };
		}
	}

	function getRemainingTTL(expiresAt: string): { label: string; isExpired: boolean } {
		try {
			const diffMs = new Date(expiresAt).getTime() - Date.now();
			if (diffMs <= 0) return { label: 'Expired', isExpired: true };
			const mins = Math.floor(diffMs / 60000);
			const secs = Math.floor((diffMs % 60000) / 1000);
			return { label: `${mins}m ${secs}s TTL`, isExpired: false };
		} catch {
			return { label: 'Active', isExpired: false };
		}
	}
</script>

<div class="space-y-4">
	<div class="flex items-center justify-between">
		<div>
			<h2 class="text-sm font-bold text-white uppercase tracking-wider font-mono">
				Proposed Remediations & HITL Approvals
			</h2>
			<p class="text-xs text-neutral-400">
				Actions catalog with explicit responder authorization gates and rollback guarantees
			</p>
		</div>

		<button
			onclick={onProposeCustom}
			class="px-3 py-1.5 rounded-lg bg-neutral-800 hover:bg-neutral-700 border border-neutral-700 text-neutral-200 text-xs font-medium transition cursor-pointer flex items-center gap-1.5 active:scale-95"
		>
			<span>+</span>
			<span>Propose Remediation</span>
		</button>
	</div>

	{#if remediations.length === 0}
		<div class="p-8 text-center rounded-xl border border-dashed border-neutral-800 text-neutral-500 text-xs font-mono">
			No remediations currently proposed.
		</div>
	{:else}
		<div class="space-y-4">
			{#each remediations as rem (rem.id)}
				{@const risk = getRiskBadge(rem.risk_level)}
				{@const ttl = getRemainingTTL(rem.expires_at)}
				<div class="rounded-xl border border-neutral-800 bg-neutral-900/40 p-5 space-y-4 backdrop-blur-sm">
					<!-- Top Row: Action, Target & Badges -->
					<div class="flex items-start sm:items-center justify-between gap-3 flex-wrap">
						<div class="flex items-center gap-2.5">
							<span class="text-xl p-1.5 rounded-lg bg-neutral-950 border border-neutral-800">
								{getActionIcon(rem.action_type)}
							</span>
							<div>
								<div class="flex items-center gap-2">
									<h3 class="text-sm font-bold text-white font-mono">
										{rem.action_type}
									</h3>
									<span class="px-2 py-0.5 rounded text-[10px] font-mono font-bold uppercase {risk.bg} {risk.border} border {risk.text}">
										{rem.risk_level} risk
									</span>
								</div>
								<p class="text-xs text-neutral-400 font-mono">
									Target: <span class="text-white font-semibold">{rem.target}</span>
								</p>
							</div>
						</div>

						<!-- Status & Expiration Pill -->
						<div class="flex items-center gap-2 font-mono text-[11px]">
							{#if rem.status === 'proposed'}
								<span class="px-2.5 py-1 rounded-md border font-medium {ttl.isExpired ? 'bg-neutral-950 border-neutral-800 text-neutral-500' : 'bg-amber-950/60 border-amber-800/80 text-amber-300'}">
									⏱ {ttl.label}
								</span>
							{/if}

							<span class="px-2.5 py-1 rounded-md border font-bold uppercase text-[10px] {
								rem.status === 'succeeded'
									? 'bg-emerald-950/80 border-emerald-800 text-emerald-300'
									: rem.status === 'proposed'
										? 'bg-amber-950/80 border-amber-800 text-amber-300'
										: rem.status === 'rejected'
											? 'bg-rose-950/80 border-rose-800 text-rose-300'
											: rem.status === 'rolled_back'
												? 'bg-neutral-800 border-neutral-700 text-neutral-300'
												: 'bg-neutral-900 border-neutral-800 text-neutral-400'
							}">
								{rem.status}
							</span>
						</div>
					</div>

					<!-- Rationale & Expected Effect Cards -->
					<div class="grid grid-cols-1 md:grid-cols-2 gap-3 text-xs">
						<div class="p-3 rounded-lg bg-neutral-950 border border-neutral-800/80 space-y-1">
							<span class="text-[10px] uppercase font-mono text-neutral-500 font-bold block">
								Rationale
							</span>
							<p class="text-neutral-300 font-sans leading-relaxed">{rem.rationale}</p>
						</div>

						<div class="p-3 rounded-lg bg-neutral-950 border border-neutral-800/80 space-y-1">
							<span class="text-[10px] uppercase font-mono text-neutral-500 font-bold block">
								Expected Outcome
							</span>
							<p class="text-emerald-300/90 font-sans leading-relaxed">{rem.expected_effect}</p>
						</div>
					</div>

					<!-- Rollback Plan -->
					{#if rem.rollback_plan}
						<div class="p-3 rounded-lg bg-neutral-950/60 border border-neutral-800/60 text-xs">
							<span class="text-[10px] uppercase font-mono text-neutral-500 font-bold block mb-1">
								Rollback / Compensation Plan
							</span>
							<p class="text-neutral-400 font-sans text-xs">{rem.rollback_plan}</p>
						</div>
					{/if}

					<!-- Parameters Payload (if any) -->
					{#if rem.params && Object.keys(rem.params).length > 0}
						<div class="space-y-1">
							<span class="text-[10px] uppercase font-mono text-neutral-500 font-bold">Execution Parameters:</span>
							<pre class="p-2 rounded bg-neutral-950 border border-neutral-800 text-[11px] font-mono text-neutral-300 overflow-x-auto">{JSON.stringify(rem.params, null, 2)}</pre>
						</div>
					{/if}

					<!-- Action Buttons Bar -->
					<div class="pt-2 border-t border-neutral-800/60 flex items-center justify-between flex-wrap gap-3">
						<span class="text-[11px] font-mono text-neutral-500">
							ID: {rem.id.slice(0, 8)}
						</span>

						<div class="flex items-center gap-2">
							{#if rem.status === 'proposed'}
								<button
									onclick={() => onReject(rem)}
									class="px-3.5 py-1.5 rounded-lg bg-rose-950/60 hover:bg-rose-900/80 border border-rose-800 text-rose-300 text-xs font-semibold transition cursor-pointer active:scale-95"
								>
									✕ Reject
								</button>
								<button
									onclick={() => onApprove(rem)}
									disabled={ttl.isExpired}
									class="px-4 py-1.5 rounded-lg bg-emerald-600 hover:bg-emerald-500 text-white text-xs font-bold transition cursor-pointer shadow-md shadow-emerald-950/50 disabled:opacity-50 active:scale-95 flex items-center gap-1.5"
								>
									<span>✓</span>
									<span>Approve & Execute</span>
								</button>
							{:else if rem.status === 'succeeded'}
								<button
									onclick={() => onRollback(rem)}
									class="px-3.5 py-1.5 rounded-lg bg-amber-950/60 hover:bg-amber-900/80 border border-amber-800 text-amber-300 text-xs font-semibold transition cursor-pointer active:scale-95"
								>
									⏪ Trigger Rollback
								</button>
							{/if}
						</div>
					</div>

					<!-- Audit Event History -->
					{#if events[rem.id] && events[rem.id].length > 0}
						<div class="pt-2 border-t border-neutral-800/40 space-y-1.5">
							<span class="text-[10px] uppercase font-mono text-neutral-500 font-semibold block">Audit Trail:</span>
							<div class="space-y-1 font-mono text-[11px]">
								{#each events[rem.id] as ev}
									<div class="flex items-center gap-2 text-neutral-400">
										<span class="text-neutral-500">{new Date(ev.created_at).toLocaleTimeString()}:</span>
										<span class="px-1.5 py-0.2 rounded bg-neutral-950 border border-neutral-800 text-neutral-300 font-bold uppercase text-[9px]">
											{ev.transition_to}
										</span>
										<span class="text-neutral-300 font-sans text-xs">{ev.reason}</span>
									</div>
								{/each}
							</div>
						</div>
					{/if}
				</div>
			{/each}
		</div>
	{/if}
</div>
