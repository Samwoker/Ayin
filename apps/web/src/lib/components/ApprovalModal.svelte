<script lang="ts">
	import type { Remediation } from '../api/types';

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
</script>

{#if mode}
	<!-- Modal Backdrop -->
	<div class="fixed inset-0 z-50 bg-neutral-950/80 backdrop-blur-sm flex items-center justify-center p-4">
		<div class="w-full max-w-lg rounded-2xl border border-neutral-800 bg-neutral-900 p-6 shadow-2xl space-y-5 animate-in fade-in duration-200">
			<!-- Modal Header -->
			<div class="flex items-start justify-between">
				<div>
					<h3 class="text-base font-bold text-white font-sans flex items-center gap-2">
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
					</h3>
					<p class="text-xs text-neutral-400 mt-1 font-sans">
						{#if mode === 'approve'}
							Human-in-the-loop verification gate. Execution will mutate target infrastructure.
						{:else if mode === 'reject'}
							Document rationale for audit compliance and agent feedback.
						{:else}
							Manually submit an action primitive to the remediation catalog.
						{/if}
					</p>
				</div>
				<button
					onclick={onClose}
					class="text-neutral-500 hover:text-white text-lg font-mono p-1 rounded transition cursor-pointer"
				>
					✕
				</button>
			</div>

			<!-- Mode: Approve -->
			{#if mode === 'approve' && remediation}
				<div class="space-y-4 text-xs">
					<!-- Action & Risk Summary Card -->
					<div class="p-3.5 rounded-xl bg-neutral-950 border border-neutral-800 space-y-2">
						<div class="flex items-center justify-between font-mono">
							<span class="font-bold text-white text-sm">{remediation.action_type}</span>
							<span class="px-2 py-0.5 rounded text-[10px] uppercase font-bold {
								remediation.risk_level === 'critical'
									? 'bg-red-950 text-red-300 border border-red-800'
									: remediation.risk_level === 'high'
										? 'bg-orange-950 text-orange-300 border border-orange-800'
										: 'bg-amber-950 text-amber-300 border border-amber-800'
							}">
								{remediation.risk_level} Risk
							</span>
						</div>
						<div class="text-neutral-400 font-mono">
							Target Workload: <span class="text-white font-semibold">{remediation.target}</span>
						</div>
						<p class="text-neutral-300 font-sans text-xs pt-1 border-t border-neutral-800/60">
							{remediation.rationale}
						</p>
					</div>

					<!-- Optional Responder Approval Notes -->
					<div class="space-y-1.5">
						<label for="approval-notes" class="text-[11px] font-mono text-neutral-400 uppercase font-semibold block">
							Approval Verification Notes (Optional):
						</label>
						<textarea
							id="approval-notes"
							bind:value={reasonText}
							placeholder="e.g. Verified with database on-call; approved for execution."
							rows="3"
							class="w-full bg-neutral-950 border border-neutral-800 rounded-lg p-3 text-xs text-white placeholder-neutral-500 focus:outline-none focus:border-emerald-500"
						></textarea>
					</div>
				</div>

				<!-- Actions -->
				<div class="flex items-center justify-end gap-3 pt-3 border-t border-neutral-800">
					<button
						onclick={onClose}
						class="px-4 py-2 rounded-lg bg-neutral-800 hover:bg-neutral-700 text-neutral-300 text-xs font-medium transition cursor-pointer"
					>
						Cancel
					</button>
					<button
						onclick={() => onConfirmApprove(reasonText)}
						disabled={isSubmitting}
						class="px-4 py-2 rounded-lg bg-emerald-600 hover:bg-emerald-500 text-white text-xs font-bold transition cursor-pointer shadow-lg shadow-emerald-950/50 flex items-center gap-1.5 disabled:opacity-50"
					>
						<span>{isSubmitting ? 'Executing...' : 'Confirm & Execute Action'}</span>
					</button>
				</div>

			<!-- Mode: Reject -->
			{:else if mode === 'reject' && remediation}
				<div class="space-y-4 text-xs">
					<div class="p-3 rounded-xl bg-neutral-950 border border-neutral-800 font-mono">
						Action: <span class="text-white font-bold">{remediation.action_type}</span> on <span class="text-white font-bold">{remediation.target}</span>
					</div>

					<div class="space-y-1.5">
						<label for="reject-reason" class="text-[11px] font-mono text-neutral-400 uppercase font-semibold block">
							Rejection Reason (Required for audit trail):
						</label>
						<textarea
							id="reject-reason"
							bind:value={reasonText}
							placeholder="e.g. Action risk unacceptable during peak traffic window; opting for traffic drain instead."
							rows="3"
							class="w-full bg-neutral-950 border border-neutral-800 rounded-lg p-3 text-xs text-white placeholder-neutral-500 focus:outline-none focus:border-rose-500"
						></textarea>
					</div>
				</div>

				<!-- Actions -->
				<div class="flex items-center justify-end gap-3 pt-3 border-t border-neutral-800">
					<button
						onclick={onClose}
						class="px-4 py-2 rounded-lg bg-neutral-800 hover:bg-neutral-700 text-neutral-300 text-xs font-medium transition cursor-pointer"
					>
						Cancel
					</button>
					<button
						onclick={() => onConfirmReject(reasonText)}
						disabled={isSubmitting || !reasonText.trim()}
						class="px-4 py-2 rounded-lg bg-rose-600 hover:bg-rose-500 text-white text-xs font-bold transition cursor-pointer shadow-lg shadow-rose-950/50 disabled:opacity-50"
					>
						<span>{isSubmitting ? 'Rejecting...' : 'Confirm Rejection'}</span>
					</button>
				</div>

			<!-- Mode: Propose -->
			{:else if mode === 'propose'}
				<div class="space-y-3.5 text-xs">
					<div class="grid grid-cols-2 gap-3">
						<div class="space-y-1">
							<label for="propose-action" class="text-[10px] uppercase font-mono text-neutral-400 font-semibold block">
								Action Type:
							</label>
							<select
								id="propose-action"
								bind:value={proposeAction}
								class="w-full bg-neutral-950 border border-neutral-800 rounded-lg p-2 text-xs text-white focus:outline-none focus:border-red-500"
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
							<label for="propose-risk" class="text-[10px] uppercase font-mono text-neutral-400 font-semibold block">
								Risk Tier:
							</label>
							<select
								id="propose-risk"
								bind:value={proposeRisk}
								class="w-full bg-neutral-950 border border-neutral-800 rounded-lg p-2 text-xs text-white focus:outline-none focus:border-red-500"
							>
								<option value="low">Low Risk</option>
								<option value="medium">Medium Risk</option>
								<option value="high">High Risk</option>
								<option value="critical">Critical Risk</option>
							</select>
						</div>
					</div>

					<div class="space-y-1">
						<label for="propose-target" class="text-[10px] uppercase font-mono text-neutral-400 font-semibold block">
							Target Workload / Service:
						</label>
						<input
							id="propose-target"
							type="text"
							bind:value={proposeTarget}
							placeholder="e.g. checkout-worker"
							class="w-full bg-neutral-950 border border-neutral-800 rounded-lg p-2 text-xs text-white placeholder-neutral-500 focus:outline-none focus:border-red-500"
						/>
					</div>

					<div class="space-y-1">
						<label for="propose-rationale" class="text-[10px] uppercase font-mono text-neutral-400 font-semibold block">
							Rationale & Mitigation Goal:
						</label>
						<textarea
							id="propose-rationale"
							bind:value={proposeRationale}
							placeholder="e.g. Scale consumers up to relieve downstream RabbitMQ queue pressure."
							rows="2"
							class="w-full bg-neutral-950 border border-neutral-800 rounded-lg p-2 text-xs text-white placeholder-neutral-500 focus:outline-none focus:border-red-500"
						></textarea>
					</div>
				</div>

				<!-- Actions -->
				<div class="flex items-center justify-end gap-3 pt-3 border-t border-neutral-800">
					<button
						onclick={onClose}
						class="px-4 py-2 rounded-lg bg-neutral-800 hover:bg-neutral-700 text-neutral-300 text-xs font-medium transition cursor-pointer"
					>
						Cancel
					</button>
					<button
						onclick={() =>
							onConfirmPropose({
								action_type: proposeAction,
								target: proposeTarget,
								rationale: proposeRationale,
								risk_level: proposeRisk
							})}
						disabled={isSubmitting || !proposeTarget.trim() || !proposeRationale.trim()}
						class="px-4 py-2 rounded-lg bg-red-600 hover:bg-red-500 text-white text-xs font-bold transition cursor-pointer shadow-lg shadow-red-950/50 disabled:opacity-50"
					>
						<span>{isSubmitting ? 'Submitting...' : 'Submit Proposal'}</span>
					</button>
				</div>
			{/if}
		</div>
	</div>
{/if}
