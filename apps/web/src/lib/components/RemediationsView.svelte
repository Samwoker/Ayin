<script lang="ts">
	import type { Remediation, RemediationEvent } from '../api/types';
	import { Button } from '#lib/components/ui/button/index.js';
	import { Badge, type BadgeVariant } from '#lib/components/ui/badge/index.js';
	import { Card } from '#lib/components/ui/card/index.js';
	import {
		ShieldAlert,
		ShieldCheck,
		RotateCcw,
		Check,
		X,
		Clock,
		Plus,
		RefreshCw,
		TrendingUp,
		Trash,
		Pause,
		Flag,
		Zap,
		SlidersHorizontal
	} from '@lucide/svelte';

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

	function getRiskBadgeVariant(risk: string): BadgeVariant {
		switch (risk?.toLowerCase()) {
			case 'critical':
				return 'sev1';
			case 'high':
				return 'sev2';
			case 'medium':
				return 'warning';
			default:
				return 'default';
		}
	}

	function getStatusBadgeVariant(status: string): BadgeVariant {
		switch (status?.toLowerCase()) {
			case 'succeeded':
				return 'default';
			case 'proposed':
				return 'warning';
			case 'rejected':
				return 'destructive';
			case 'rolled_back':
				return 'secondary';
			default:
				return 'outline';
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

{#snippet renderActionIcon(action: string)}
	{#if action === 'restart_service'}
		<RefreshCw class="w-4 h-4 text-sky-400" />
	{:else if action === 'scale_service'}
		<TrendingUp class="w-4 h-4 text-emerald-400" />
	{:else if action === 'rollback_deploy'}
		<RotateCcw class="w-4 h-4 text-amber-400" />
	{:else if action === 'clear_cache'}
		<Trash class="w-4 h-4 text-orange-400" />
	{:else if action === 'pause_consumer'}
		<Pause class="w-4 h-4 text-purple-400" />
	{:else if action === 'toggle_feature_flag'}
		<Flag class="w-4 h-4 text-rose-400" />
	{:else}
		<Zap class="w-4 h-4 text-yellow-400" />
	{/if}
{/snippet}

<div class="space-y-4">
	<div class="flex items-center justify-between">
		<div>
			<h2 class="text-sm font-bold text-white uppercase tracking-wider font-mono flex items-center gap-2">
				<ShieldAlert class="w-4 h-4 text-amber-400" />
				<span>Proposed Remediations & HITL Approvals</span>
			</h2>
			<p class="text-xs text-slate-400 font-sans">
				Actions catalog with explicit responder authorization gates and rollback guarantees
			</p>
		</div>

		<Button
			variant="outline"
			size="sm"
			onclick={onProposeCustom}
			class="gap-1.5 border-slate-700 bg-slate-900/80 hover:bg-slate-800 text-slate-200"
		>
			<Plus class="w-3.5 h-3.5" />
			<span>Propose Remediation</span>
		</Button>
	</div>

	{#if remediations.length === 0}
		<div class="p-8 text-center rounded-xl border border-dashed border-slate-800 text-slate-500 text-xs font-mono">
			No remediations currently proposed.
		</div>
	{:else}
		<div class="space-y-4">
			{#each remediations as rem (rem.id)}
				{@const ttl = getRemainingTTL(rem.expires_at)}
				<Card class="p-5 space-y-4 border-slate-800/80 bg-slate-900/60 backdrop-blur-sm shadow-sm hover:border-slate-700/80 transition-all">
					<!-- Top Row: Action, Target & Badges -->
					<div class="flex items-start sm:items-center justify-between gap-3 flex-wrap">
						<div class="flex items-center gap-3">
							<div class="p-2 rounded-xl bg-slate-950 border border-slate-800 shadow-inner flex items-center justify-center">
								{@render renderActionIcon(rem.action_type)}
							</div>
							<div>
								<div class="flex items-center gap-2">
									<h3 class="text-sm font-bold text-white font-mono">
										{rem.action_type}
									</h3>
									<Badge variant={getRiskBadgeVariant(rem.risk_level)} class="uppercase font-mono text-[10px]">
										{rem.risk_level} risk
									</Badge>
								</div>
								<p class="text-xs text-slate-400 font-mono">
									Target: <span class="text-white font-semibold">{rem.target}</span>
								</p>
							</div>
						</div>

						<!-- Status & Expiration Pill -->
						<div class="flex items-center gap-2 font-mono text-[11px]">
							{#if rem.status === 'proposed'}
								<Badge variant={ttl.isExpired ? 'secondary' : 'warning'} class="text-[11px] font-mono gap-1">
									<Clock class="w-3 h-3" />
									<span>{ttl.label}</span>
								</Badge>
							{/if}

							<Badge variant={getStatusBadgeVariant(rem.status)} class="uppercase font-mono text-[10px] font-bold">
								{rem.status}
							</Badge>
						</div>
					</div>

					<!-- Rationale & Expected Effect Cards -->
					<div class="grid grid-cols-1 md:grid-cols-2 gap-3 text-xs">
						<div class="p-3.5 rounded-lg bg-slate-950/80 border border-slate-800/80 space-y-1">
							<span class="text-[10px] uppercase font-mono text-slate-500 font-bold block">
								Rationale
							</span>
							<p class="text-slate-300 font-sans leading-relaxed">{rem.rationale}</p>
						</div>

						<div class="p-3.5 rounded-lg bg-slate-950/80 border border-slate-800/80 space-y-1">
							<span class="text-[10px] uppercase font-mono text-slate-500 font-bold block">
								Expected Outcome
							</span>
							<p class="text-emerald-300/90 font-sans leading-relaxed">{rem.expected_effect}</p>
						</div>
					</div>

					<!-- Rollback Plan -->
					{#if rem.rollback_plan}
						<div class="p-3 rounded-lg bg-slate-950/60 border border-slate-800/60 text-xs">
							<span class="text-[10px] uppercase font-mono text-slate-500 font-bold block mb-1">
								Rollback / Compensation Plan
							</span>
							<p class="text-slate-400 font-sans text-xs">{rem.rollback_plan}</p>
						</div>
					{/if}

					<!-- Parameters Payload (if any) -->
					{#if rem.params && Object.keys(rem.params).length > 0}
						<div class="space-y-1">
							<span class="text-[10px] uppercase font-mono text-slate-500 font-bold flex items-center gap-1">
								<SlidersHorizontal class="w-3 h-3 text-slate-400" />
								<span>Execution Parameters:</span>
							</span>
							<pre class="p-2.5 rounded-lg bg-slate-950 border border-slate-800 text-[11px] font-mono text-slate-300 overflow-x-auto">{JSON.stringify(rem.params, null, 2)}</pre>
						</div>
					{/if}

					<!-- Action Buttons Bar -->
					<div class="pt-2 border-t border-slate-800/60 flex items-center justify-between flex-wrap gap-3">
						<span class="text-[11px] font-mono text-slate-500">
							ID: #{rem.id.slice(0, 8)}
						</span>

						<div class="flex items-center gap-2">
							{#if rem.status === 'proposed'}
								<Button
									variant="destructive"
									size="sm"
									onclick={() => onReject(rem)}
									class="gap-1.5"
								>
									<X class="w-3.5 h-3.5" />
									<span>Reject</span>
								</Button>
								<Button
									variant="default"
									size="sm"
									onclick={() => onApprove(rem)}
									disabled={ttl.isExpired}
									class="gap-1.5 shadow-sm active:scale-95"
								>
									<Check class="w-3.5 h-3.5" />
									<span>Approve & Execute</span>
								</Button>
							{:else if rem.status === 'succeeded'}
								<Button
									variant="outline"
									size="sm"
									onclick={() => onRollback(rem)}
									class="text-amber-400 hover:text-amber-300 border-amber-800/60 gap-1.5"
								>
									<RotateCcw class="w-3.5 h-3.5" />
									<span>Trigger Rollback</span>
								</Button>
							{/if}
						</div>
					</div>

					<!-- Audit Event History -->
					{#if events[rem.id] && events[rem.id].length > 0}
						<div class="pt-2 border-t border-slate-800/40 space-y-1.5">
							<span class="text-[10px] uppercase font-mono text-slate-500 font-semibold block">Audit Trail:</span>
							<div class="space-y-1 font-mono text-[11px]">
								{#each events[rem.id] as ev}
									<div class="flex items-center gap-2 text-slate-400">
										<span class="text-slate-500">{new Date(ev.created_at).toLocaleTimeString()}:</span>
										<Badge variant="outline" class="text-[9px] uppercase font-bold py-0 px-1.5">
											{ev.transition_to}
										</Badge>
										<span class="text-slate-300 font-sans text-xs">{ev.reason}</span>
									</div>
								{/each}
							</div>
						</div>
					{/if}
				</Card>
			{/each}
		</div>
	{/if}
</div>
