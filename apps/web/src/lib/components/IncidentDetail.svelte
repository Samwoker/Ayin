<script lang="ts">
	import type {
		Incident,
		Run,
		RunStep,
		Remediation,
		RemediationEvent,
		Runbook
	} from '../api/types';
	import type { HypothesisData } from '../demo-data';

	import InvestigationTrace from './InvestigationTrace.svelte';
	import HypothesesView from './HypothesesView.svelte';
	import RemediationsView from './RemediationsView.svelte';
	import RunbookViewer from './RunbookViewer.svelte';
	import ApprovalModal from './ApprovalModal.svelte';

	interface Props {
		incident: Incident;
		run: Run | null;
		steps: RunStep[];
		hypotheses: HypothesisData[];
		remediations: Remediation[];
		remediationEvents: Record<string, RemediationEvent[]>;
		runbooks: Runbook[];
		isLiveStreaming: boolean;
		onStatusChange: (newStatus: string) => void;
		onTriggerRun: () => void;
		onApproveRemediation: (rem: Remediation, reason: string) => Promise<void>;
		onRejectRemediation: (rem: Remediation, reason: string) => Promise<void>;
		onRollbackRemediation: (rem: Remediation, reason: string) => Promise<void>;
		onProposeRemediation: (payload: {
			action_type: string;
			target: string;
			rationale: string;
			risk_level: string;
		}) => Promise<void>;
		isTriggering: boolean;
	}

	let {
		incident,
		run = null,
		steps = [],
		hypotheses = [],
		remediations = [],
		remediationEvents = {},
		runbooks = [],
		isLiveStreaming = false,
		onStatusChange,
		onTriggerRun,
		onApproveRemediation,
		onRejectRemediation,
		onRollbackRemediation,
		onProposeRemediation,
		isTriggering = false
	}: Props = $props();

	type TabType = 'trace' | 'hypotheses' | 'remediations' | 'runbooks';
	let activeTab = $state<TabType>('trace');

	// Modal State
	let modalMode = $state<'approve' | 'reject' | 'propose' | null>(null);
	let targetRemediation = $state<Remediation | null>(null);
	let isModalSubmitting = $state(false);

	function openApproveModal(rem: Remediation) {
		targetRemediation = rem;
		modalMode = 'approve';
	}

	function openRejectModal(rem: Remediation) {
		targetRemediation = rem;
		modalMode = 'reject';
	}

	function openProposeModal() {
		modalMode = 'propose';
	}

	function closeModal() {
		modalMode = null;
		targetRemediation = null;
	}

	async function handleConfirmApprove(reason: string) {
		if (!targetRemediation) return;
		isModalSubmitting = true;
		try {
			await onApproveRemediation(targetRemediation, reason);
			closeModal();
		} finally {
			isModalSubmitting = false;
		}
	}

	async function handleConfirmReject(reason: string) {
		if (!targetRemediation) return;
		isModalSubmitting = true;
		try {
			await onRejectRemediation(targetRemediation, reason);
			closeModal();
		} finally {
			isModalSubmitting = false;
		}
	}

	async function handleConfirmPropose(payload: {
		action_type: string;
		target: string;
		rationale: string;
		risk_level: string;
	}) {
		isModalSubmitting = true;
		try {
			await onProposeRemediation(payload);
			closeModal();
		} finally {
			isModalSubmitting = false;
		}
	}
</script>

<div class="rounded-xl border border-neutral-800 bg-neutral-900/40 backdrop-blur-sm overflow-hidden flex flex-col">
	<!-- Incident Master Header -->
	<div class="p-6 border-b border-neutral-800/80 bg-neutral-900/60 space-y-4">
		<div class="flex items-start justify-between gap-4 flex-wrap">
			<div class="space-y-1.5 max-w-2xl">
				<div class="flex items-center gap-2.5">
					<!-- Severity Badge -->
					<span class="px-2.5 py-0.5 rounded font-mono font-bold text-xs uppercase {
						incident.severity === 'sev1'
							? 'bg-red-950 text-red-300 border border-red-800'
							: incident.severity === 'sev2'
								? 'bg-amber-950 text-amber-300 border border-amber-800'
								: incident.severity === 'sev3'
									? 'bg-yellow-950 text-yellow-300 border border-yellow-800'
									: 'bg-blue-950 text-blue-300 border border-blue-800'
					}">
						{incident.severity}
					</span>

					<!-- Service Tag -->
					<span class="px-2.5 py-0.5 rounded bg-neutral-950 border border-neutral-800 text-neutral-300 font-mono text-xs font-semibold">
						{incident.service}
					</span>

					<span class="text-xs text-neutral-500 font-mono">
						Incident #{incident.id.slice(0, 8)}
					</span>
				</div>

				<h2 class="text-xl font-bold text-white tracking-tight leading-snug">
					{incident.title}
				</h2>
			</div>

			<!-- Status Selector Dropdown -->
			<div class="flex items-center gap-2 font-mono text-xs">
				<span class="text-neutral-500">Status:</span>
				<select
					value={incident.status}
					onchange={(e) => onStatusChange((e.target as HTMLSelectElement).value)}
					class="bg-neutral-950 border border-neutral-800 rounded-lg px-3 py-1.5 text-xs font-semibold text-white focus:outline-none focus:border-red-500 cursor-pointer capitalize"
				>
					<option value="open">Open</option>
					<option value="investigating">Investigating</option>
					<option value="mitigated">Mitigated</option>
					<option value="resolved">Resolved</option>
				</select>
			</div>
		</div>

		<!-- Navigation Tabs -->
		<div class="flex items-center gap-2 pt-2 border-t border-neutral-800/60 overflow-x-auto text-xs font-mono">
			<button
				onclick={() => (activeTab = 'trace')}
				class="px-4 py-2 rounded-lg transition cursor-pointer flex items-center gap-2 {activeTab === 'trace' ? 'bg-neutral-800 text-white font-bold shadow-sm' : 'text-neutral-400 hover:text-white'}"
			>
				<span>🤖</span>
				<span>Investigation Trace</span>
				{#if steps.length > 0}
					<span class="px-1.5 py-0.2 rounded-full bg-neutral-900 text-[10px] text-neutral-400">
						{steps.length}
					</span>
				{/if}
			</button>

			<button
				onclick={() => (activeTab = 'hypotheses')}
				class="px-4 py-2 rounded-lg transition cursor-pointer flex items-center gap-2 {activeTab === 'hypotheses' ? 'bg-neutral-800 text-white font-bold shadow-sm' : 'text-neutral-400 hover:text-white'}"
			>
				<span>💡</span>
				<span>Ranked Hypotheses</span>
				{#if hypotheses.length > 0}
					<span class="px-1.5 py-0.2 rounded-full bg-neutral-900 text-[10px] text-indigo-400 font-bold">
						{hypotheses.length}
					</span>
				{/if}
			</button>

			<button
				onclick={() => (activeTab = 'remediations')}
				class="px-4 py-2 rounded-lg transition cursor-pointer flex items-center gap-2 {activeTab === 'remediations' ? 'bg-neutral-800 text-white font-bold shadow-sm' : 'text-neutral-400 hover:text-white'}"
			>
				<span>🛡️</span>
				<span>Remediations & Approvals</span>
				{#if remediations.length > 0}
					<span class="px-1.5 py-0.2 rounded-full bg-amber-950 border border-amber-800/60 text-[10px] text-amber-300 font-bold">
						{remediations.length}
					</span>
				{/if}
			</button>

			<button
				onclick={() => (activeTab = 'runbooks')}
				class="px-4 py-2 rounded-lg transition cursor-pointer flex items-center gap-2 {activeTab === 'runbooks' ? 'bg-neutral-800 text-white font-bold shadow-sm' : 'text-neutral-400 hover:text-white'}"
			>
				<span>📖</span>
				<span>Runbooks</span>
			</button>
		</div>
	</div>

	<!-- Tab Panels -->
	<div class="p-6">
		{#if activeTab === 'trace'}
			<InvestigationTrace
				{run}
				{steps}
				{isLiveStreaming}
				{onTriggerRun}
				{isTriggering}
			/>
		{:else if activeTab === 'hypotheses'}
			<HypothesesView {hypotheses} />
		{:else if activeTab === 'remediations'}
			<RemediationsView
				{remediations}
				events={remediationEvents}
				onApprove={openApproveModal}
				onReject={openRejectModal}
				onRollback={(rem) => onRollbackRemediation(rem, 'Responder manual rollback')}
				onProposeCustom={openProposeModal}
			/>
		{:else if activeTab === 'runbooks'}
			<RunbookViewer {runbooks} targetService={incident.service} />
		{/if}
	</div>
</div>

<!-- Modal Controls -->
<ApprovalModal
	mode={modalMode}
	remediation={targetRemediation}
	onClose={closeModal}
	onConfirmApprove={handleConfirmApprove}
	onConfirmReject={handleConfirmReject}
	onConfirmPropose={handleConfirmPropose}
	isSubmitting={isModalSubmitting}
/>
