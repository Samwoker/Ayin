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
	import { Card } from '#lib/components/ui/card/index.js';
	import { Badge, type BadgeVariant } from '#lib/components/ui/badge/index.js';
	import { Tabs, TabsList, TabsTrigger } from '#lib/components/ui/tabs/index.js';
	import { Bot, Sparkles, ShieldCheck, BookOpen, Flame, Tag } from '@lucide/svelte';

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
	let activeTab = $state<string>('trace');

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

	function getSeverityBadgeVariant(severity: string): BadgeVariant {
		switch (severity?.toLowerCase()) {
			case 'sev1':
				return 'sev1';
			case 'sev2':
				return 'sev2';
			case 'sev3':
				return 'sev3';
			case 'sev4':
				return 'sev4';
			default:
				return 'secondary';
		}
	}
</script>

<Card class="border-slate-800/80 bg-slate-900/40 backdrop-blur-sm overflow-hidden flex flex-col shadow-sm">
	<!-- Incident Master Header -->
	<div class="p-6 border-b border-slate-800/80 bg-slate-900/60 space-y-4">
		<div class="flex items-start justify-between gap-4 flex-wrap">
			<div class="space-y-2 max-w-2xl">
				<div class="flex items-center gap-2.5 flex-wrap">
					<!-- Severity Badge -->
					<Badge variant={getSeverityBadgeVariant(incident.severity)} class="uppercase font-mono text-xs px-2.5 py-0.5">
						{#if incident.severity === 'sev1'}
							<Flame class="w-3 h-3 text-red-400 inline -ml-0.5 mr-0.5" />
						{/if}
						{incident.severity}
					</Badge>

					<!-- Service Tag -->
					<span class="px-2.5 py-0.5 rounded bg-slate-950 border border-slate-800 text-slate-300 font-mono text-xs font-semibold flex items-center gap-1.5">
						<Tag class="w-3 h-3 text-slate-400" />
						<span>{incident.service}</span>
					</span>

					<span class="text-xs text-slate-500 font-mono">
						Incident #{incident.id.slice(0, 8)}
					</span>
				</div>

				<h2 class="text-xl font-bold text-white tracking-tight leading-snug font-sans">
					{incident.title}
				</h2>
			</div>

			<!-- Status Selector Dropdown -->
			<div class="flex items-center gap-2 font-mono text-xs">
				<span class="text-slate-400">Status:</span>
				<select
					value={incident.status}
					onchange={(e) => onStatusChange((e.target as HTMLSelectElement).value)}
					class="bg-slate-950 border border-slate-800 rounded-lg px-3 py-1.5 text-xs font-semibold text-white focus:outline-none focus:ring-1 focus:ring-emerald-500/50 cursor-pointer capitalize shadow-sm transition"
				>
					<option value="open">Open</option>
					<option value="investigating">Investigating</option>
					<option value="mitigated">Mitigated</option>
					<option value="resolved">Resolved</option>
				</select>
			</div>
		</div>

		<!-- Navigation Tabs using shadcn Tabs -->
		<Tabs bind:value={activeTab} class="w-full">
			<TabsList class="w-full justify-start h-auto p-1 bg-slate-950/90 border-slate-800 gap-1.5 overflow-x-auto">
				<TabsTrigger value="trace" class="gap-2 text-xs py-2 px-3.5">
					<Bot class="w-3.5 h-3.5" />
					<span>Investigation Trace</span>
					{#if steps.length > 0}
						<Badge variant="secondary" class="text-[10px] px-1.5 py-0 h-4 font-mono">
							{steps.length}
						</Badge>
					{/if}
				</TabsTrigger>

				<TabsTrigger value="hypotheses" class="gap-2 text-xs py-2 px-3.5">
					<Sparkles class="w-3.5 h-3.5" />
					<span>Ranked Hypotheses</span>
					{#if hypotheses.length > 0}
						<Badge variant="secondary" class="text-[10px] text-indigo-400 px-1.5 py-0 h-4 font-mono">
							{hypotheses.length}
						</Badge>
					{/if}
				</TabsTrigger>

				<TabsTrigger value="remediations" class="gap-2 text-xs py-2 px-3.5">
					<ShieldCheck class="w-3.5 h-3.5" />
					<span>Remediations & Approvals</span>
					{#if remediations.length > 0}
						<Badge variant="warning" class="text-[10px] px-1.5 py-0 h-4 font-mono">
							{remediations.length}
						</Badge>
					{/if}
				</TabsTrigger>

				<TabsTrigger value="runbooks" class="gap-2 text-xs py-2 px-3.5">
					<BookOpen class="w-3.5 h-3.5" />
					<span>Runbooks</span>
				</TabsTrigger>
			</TabsList>
		</Tabs>
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
</Card>

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
