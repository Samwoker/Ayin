<script lang="ts">
	import type { Remediation } from '../api/types';
	import {
		Dialog,
		DialogContent,
		DialogDescription,
		DialogFooter,
		DialogHeader,
		DialogTitle
	} from '#lib/components/ui/dialog/index.js';
	import { Button } from '#lib/components/ui/button/index.js';
	import { Badge } from '#lib/components/ui/badge/index.js';
	import { Input } from '#lib/components/ui/input/index.js';
	import { Textarea } from '#lib/components/ui/textarea/index.js';
	import { Card } from '#lib/components/ui/card/index.js';

	interface Props {
		mode: 'approve' | 'reject' | 'propose' | null;
		remediation: Remediation | null;
		onClose: () => void;
		onConfirmApprove: (reason: string) => void;
		onConfirmReject: (reason: string) => void;
		onConfirmPropose: (payload: {
			action_type: string;
			target: string;
			rationale: string;
			risk_level: string;
		}) => void;
		isSubmitting: boolean;
	}

	let {
		mode = null,
		remediation = null,
		onClose,
		onConfirmApprove,
		onConfirmReject,
		onConfirmPropose,
		isSubmitting = false
	}: Props = $props();

	let reasonText = $state('');

	// Propose form state
	let proposeAction = $state('scale_service');
	let proposeTarget = $state('');
	let proposeRationale = $state('');
	let proposeRisk = $state('medium');

	let isOpen = $derived(mode !== null);

	function handleOpenChange(open: boolean) {
		if (!open) {
			onClose();
		}
	}
</script>

<Dialog open={isOpen} onOpenChange={handleOpenChange}>
	<DialogContent class="max-w-lg bg-slate-900 border-slate-800 text-slate-100 shadow-2xl">
		<DialogHeader>
			<DialogTitle class="flex items-center gap-2 text-base font-bold font-sans">
				{#if mode === 'approve'}
					<span class="text-emerald-400">🛡️</span>
					<span>Authorize Remediation Execution</span>
				{:else if mode === 'reject'}
					<span class="text-rose-400">✕</span>
					<span>Reject Proposed Remediation</span>
				{:else}
					<span>⚡</span>
					<span>Propose New Incident Remediation</span>
				{/if}
			</DialogTitle>
			<DialogDescription class="text-xs text-slate-400 font-sans">
				{#if mode === 'approve'}
					Human-in-the-loop verification gate. Execution will mutate target infrastructure.
				{:else if mode === 'reject'}
					Document rationale for audit compliance and agent feedback.
				{:else}
					Manually submit an action primitive to the remediation catalog.
				{/if}
			</DialogDescription>
		</DialogHeader>

		<!-- Mode: Approve -->
		{#if mode === 'approve' && remediation}
			<div class="space-y-4 text-xs">
				<!-- Action & Risk Summary Card -->
				<Card class="p-3.5 bg-slate-950/80 border-slate-800 space-y-2">
					<div class="flex items-center justify-between font-mono">
						<span class="font-bold text-white text-sm">{remediation.action_type}</span>
						<Badge
							variant={remediation.risk_level === 'critical'
								? 'sev1'
								: remediation.risk_level === 'high'
									? 'sev2'
									: 'warning'}
						>
							{remediation.risk_level} Risk
						</Badge>
					</div>
					<div class="text-slate-400 font-mono">
						Target Workload: <span class="text-white font-semibold">{remediation.target}</span>
					</div>
					<p class="text-slate-300 font-sans text-xs pt-1 border-t border-slate-800/60">
						{remediation.rationale}
					</p>
				</Card>

				<!-- Optional Responder Approval Notes -->
				<div class="space-y-1.5">
					<label for="approval-notes" class="text-[11px] font-mono text-slate-400 uppercase font-semibold block">
						Approval Verification Notes (Optional):
					</label>
					<Textarea
						id="approval-notes"
						bind:value={reasonText}
						placeholder="e.g. Verified with database on-call; approved for execution."
						rows={3}
					/>
				</div>
			</div>

			<DialogFooter class="gap-2 sm:gap-0 pt-3 border-t border-slate-800">
				<Button variant="secondary" onclick={onClose} disabled={isSubmitting}>
					Cancel
				</Button>
				<Button
					variant="default"
					onclick={() => onConfirmApprove(reasonText)}
					disabled={isSubmitting}
					class="gap-1.5"
				>
					<span>{isSubmitting ? 'Executing...' : 'Confirm & Execute Action'}</span>
				</Button>
			</DialogFooter>

		<!-- Mode: Reject -->
		{:else if mode === 'reject' && remediation}
			<div class="space-y-4 text-xs">
				<Card class="p-3 bg-slate-950/80 border-slate-800 font-mono">
					Action: <span class="text-white font-bold">{remediation.action_type}</span> on <span class="text-white font-bold">{remediation.target}</span>
				</Card>

				<div class="space-y-1.5">
					<label for="reject-reason" class="text-[11px] font-mono text-slate-400 uppercase font-semibold block">
						Rejection Reason (Required for audit trail):
					</label>
					<Textarea
						id="reject-reason"
						bind:value={reasonText}
						placeholder="e.g. Action risk unacceptable during peak traffic window; opting for traffic drain instead."
						rows={3}
					/>
				</div>
			</div>

			<DialogFooter class="gap-2 sm:gap-0 pt-3 border-t border-slate-800">
				<Button variant="secondary" onclick={onClose} disabled={isSubmitting}>
					Cancel
				</Button>
				<Button
					variant="destructive"
					onclick={() => onConfirmReject(reasonText)}
					disabled={isSubmitting || !reasonText.trim()}
				>
					<span>{isSubmitting ? 'Rejecting...' : 'Confirm Rejection'}</span>
				</Button>
			</DialogFooter>

		<!-- Mode: Propose -->
		{:else if mode === 'propose'}
			<div class="space-y-3.5 text-xs">
				<div class="grid grid-cols-2 gap-3">
					<div class="space-y-1">
						<label for="propose-action" class="text-[10px] uppercase font-mono text-slate-400 font-semibold block">
							Action Type:
						</label>
						<select
							id="propose-action"
							bind:value={proposeAction}
							class="w-full bg-slate-950 border border-slate-700 rounded-md p-2 text-xs text-white focus:outline-none focus:border-emerald-500"
						>
							<option value="scale_service">scale_service</option>
							<option value="restart_service">restart_service</option>
							<option value="rollback_deploy">rollback_deploy</option>
							<option value="clear_cache">clear_cache</option>
							<option value="pause_consumer">pause_consumer</option>
							<option value="toggle_feature_flag">toggle_feature_flag</option>
						</select>
					</div>

					<div class="space-y-1">
						<label for="propose-risk" class="text-[10px] uppercase font-mono text-slate-400 font-semibold block">
							Risk Tier:
						</label>
						<select
							id="propose-risk"
							bind:value={proposeRisk}
							class="w-full bg-slate-950 border border-slate-700 rounded-md p-2 text-xs text-white focus:outline-none focus:border-emerald-500"
						>
							<option value="low">Low Risk</option>
							<option value="medium">Medium Risk</option>
							<option value="high">High Risk</option>
							<option value="critical">Critical Risk</option>
						</select>
					</div>
				</div>

				<div class="space-y-1">
					<label for="propose-target" class="text-[10px] uppercase font-mono text-slate-400 font-semibold block">
						Target Workload / Service:
					</label>
					<Input
						id="propose-target"
						bind:value={proposeTarget}
						placeholder="e.g. checkout-worker"
					/>
				</div>

				<div class="space-y-1">
					<label for="propose-rationale" class="text-[10px] uppercase font-mono text-slate-400 font-semibold block">
						Rationale & Mitigation Goal:
					</label>
					<Textarea
						id="propose-rationale"
						bind:value={proposeRationale}
						placeholder="e.g. Scale consumers up to relieve downstream RabbitMQ queue pressure."
						rows={2}
					/>
				</div>
			</div>

			<DialogFooter class="gap-2 sm:gap-0 pt-3 border-t border-slate-800">
				<Button variant="secondary" onclick={onClose} disabled={isSubmitting}>
					Cancel
				</Button>
				<Button
					variant="destructive"
					onclick={() =>
						onConfirmPropose({
							action_type: proposeAction,
							target: proposeTarget,
							rationale: proposeRationale,
							risk_level: proposeRisk
						})}
					disabled={isSubmitting || !proposeTarget.trim() || !proposeRationale.trim()}
				>
					<span>{isSubmitting ? 'Submitting...' : 'Submit Proposal'}</span>
				</Button>
			</DialogFooter>
		{/if}
	</DialogContent>
</Dialog>
