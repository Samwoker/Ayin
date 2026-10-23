<script lang="ts">
	import type { Incident } from '../api/types';
	import { Input } from '#lib/components/ui/input/index.js';
	import {
		Search,
		Clock,
		Flame,
		AlertTriangle,
		AlertCircle,
		CheckCircle2,
		Inbox,
		Radio,
		Layers
	} from '@lucide/svelte';

	interface Props {
		incidents: Incident[];
		selectedIncidentId: string | null;
		onSelect: (incident: Incident) => void;
	}

	let {
		incidents = [],
		selectedIncidentId = null,
		onSelect
	}: Props = $props();

	let searchQuery = $state('');
	let selectedSeverity = $state('all');
	let selectedStatusFilter = $state('all');

	let filteredIncidents = $derived(
		incidents.filter((inc) => {
			const matchesSearch =
				searchQuery === '' ||
				inc.title.toLowerCase().includes(searchQuery.toLowerCase()) ||
				inc.service.toLowerCase().includes(searchQuery.toLowerCase());

			const matchesSeverity =
				selectedSeverity === 'all' || inc.severity.toLowerCase() === selectedSeverity;

			const matchesStatus =
				selectedStatusFilter === 'all' ||
				(selectedStatusFilter === 'active' &&
					(inc.status === 'open' || inc.status === 'investigating')) ||
				(selectedStatusFilter === 'resolved' &&
					(inc.status === 'resolved' || inc.status === 'mitigated'));

			return matchesSearch && matchesSeverity && matchesStatus;
		})
	);

	function formatTimeAgo(isoString?: string): string {
		if (!isoString) return 'recent';
		try {
			const diffMs = Date.now() - new Date(isoString).getTime();
			const mins = Math.floor(diffMs / 60000);
			if (mins < 1) return 'just now';
			if (mins < 60) return `${mins}m ago`;
			const hours = Math.floor(mins / 60);
			if (hours < 24) return `${hours}h ago`;
			return `${Math.floor(hours / 24)}d ago`;
		} catch {
			return 'recent';
		}
	}

	function getSeverityBadge(severity: string) {
		switch (severity?.toLowerCase()) {
			case 'sev1':
				return {
					label: 'SEV1',
					dot: 'bg-rose-500 shadow-rose-500/50',
					border: 'border-rose-500/40 text-rose-300 bg-rose-950/40'
				};
			case 'sev2':
				return {
					label: 'SEV2',
					dot: 'bg-amber-400 shadow-amber-400/50',
					border: 'border-amber-500/40 text-amber-300 bg-amber-950/40'
				};
			case 'sev3':
				return {
					label: 'SEV3',
					dot: 'bg-yellow-400 shadow-yellow-400/50',
					border: 'border-yellow-500/40 text-yellow-300 bg-yellow-950/40'
				};
			default:
				return {
					label: 'SEV4',
					dot: 'bg-slate-400 shadow-slate-400/50',
					border: 'border-slate-700 text-slate-300 bg-slate-900/60'
				};
		}
	}
</script>

<div class="rounded-xl border border-white/[0.08] bg-[#0d0f14]/90 backdrop-blur-md p-4 space-y-3.5 shadow-lg shadow-black/40">
	<!-- Queue Header & Status Filter Pills -->
	<div class="space-y-3">
		<div class="flex items-center justify-between">
			<div class="flex items-center gap-2">
				<Layers class="w-4 h-4 text-indigo-400" />
				<h2 class="text-xs font-bold text-slate-200 uppercase tracking-wider font-mono">
					Incident Queue
				</h2>
				<span class="text-[10px] font-mono px-1.5 py-0.5 rounded-full bg-white/[0.05] border border-white/10 text-slate-400">
					{filteredIncidents.length}
				</span>
			</div>

			<!-- Status Filter Tabs (Segmented) -->
			<div class="flex items-center rounded-lg bg-black/50 p-0.5 border border-white/[0.06] text-[11px] font-mono">
				<button
					onclick={() => (selectedStatusFilter = 'all')}
					class="px-2.5 py-0.8 rounded-md transition cursor-pointer {selectedStatusFilter === 'all' ? 'bg-white/[0.08] text-white font-semibold' : 'text-slate-400 hover:text-slate-200'}"
				>
					All
				</button>
				<button
					onclick={() => (selectedStatusFilter = 'active')}
					class="px-2.5 py-0.8 rounded-md transition cursor-pointer {selectedStatusFilter === 'active' ? 'bg-rose-500/20 text-rose-300 font-semibold' : 'text-slate-400 hover:text-slate-200'}"
				>
					Active
				</button>
				<button
					onclick={() => (selectedStatusFilter = 'resolved')}
					class="px-2.5 py-0.8 rounded-md transition cursor-pointer {selectedStatusFilter === 'resolved' ? 'bg-emerald-500/20 text-emerald-300 font-semibold' : 'text-slate-400 hover:text-slate-200'}"
				>
					Resolved
				</button>
			</div>
		</div>

		<!-- Fast Search Input -->
		<div class="relative">
			<Search class="w-3.5 h-3.5 text-slate-500 absolute left-3 top-2.5 pointer-events-none" />
			<Input
				placeholder="Search by title, service, error..."
				bind:value={searchQuery}
				class="pl-8 pr-12 bg-black/40 border-white/[0.08] text-xs h-8.5 rounded-lg text-slate-200 placeholder:text-slate-500 focus-visible:ring-indigo-500/40 focus-visible:border-indigo-500/50"
			/>
			<span class="absolute right-2.5 top-2 text-[10px] font-mono text-slate-600 border border-white/[0.08] rounded px-1 bg-white/[0.02]">
				⌘K
			</span>
		</div>

		<!-- Severity Tiers Filter Pills -->
		<div class="flex items-center gap-1 overflow-x-auto text-[10px] font-mono pb-0.5">
			{#each ['all', 'sev1', 'sev2', 'sev3', 'sev4'] as sev}
				<button
					onclick={() => (selectedSeverity = sev)}
					class="px-2.5 py-0.5 rounded-md border transition cursor-pointer uppercase {selectedSeverity === sev ? 'bg-indigo-600/20 border-indigo-500/50 text-indigo-200 font-bold' : 'bg-black/30 border-white/[0.05] text-slate-400 hover:text-slate-200 hover:border-white/10'}"
				>
					{sev}
				</button>
			{/each}
		</div>
	</div>

	<!-- Incidents Feed -->
	<div class="space-y-1.5 max-h-[640px] overflow-y-auto pr-0.5">
		{#if filteredIncidents.length === 0}
			<div class="p-8 text-center rounded-xl border border-dashed border-white/10 text-slate-500 text-xs font-mono space-y-2">
				<Inbox class="w-6 h-6 mx-auto text-slate-600" />
				<div>No incidents match filter</div>
			</div>
		{:else}
			{#each filteredIncidents as inc (inc.id)}
				{@const isSelected = selectedIncidentId === inc.id}
				{@const sevConfig = getSeverityBadge(inc.severity)}
				<button
					type="button"
					onclick={() => onSelect(inc)}
					class="w-full text-left p-3 rounded-xl border transition-all cursor-pointer text-xs space-y-2 relative overflow-hidden group {isSelected ? 'bg-[#151821] border-indigo-500/40 shadow-md shadow-indigo-950/20' : 'bg-black/20 border-white/[0.05] hover:bg-[#13151b] hover:border-white/10'}"
				>
					<!-- Selected Left Edge Rail -->
					{#if isSelected}
						<div class="absolute left-0 top-0 bottom-0 w-1 bg-indigo-500 shadow-sm shadow-indigo-500/50"></div>
					{/if}

					<!-- Top Row: Severity + Service Tag + Timestamp -->
					<div class="flex items-center justify-between gap-2">
						<div class="flex items-center gap-1.5">
							<span class="px-1.5 py-0.2 rounded font-mono text-[9px] font-bold border uppercase {sevConfig.border}">
								{sevConfig.label}
							</span>
							<span class="px-2 py-0.5 rounded bg-white/[0.04] border border-white/[0.06] font-mono text-[10px] text-slate-300 font-medium">
								{inc.service}
							</span>
						</div>

						<span class="text-[10px] font-mono text-slate-500 flex items-center gap-1">
							<Clock class="w-3 h-3 text-slate-600" />
							{formatTimeAgo(inc.opened_at || inc.created_at)}
						</span>
					</div>

					<!-- Title -->
					<h3 class="text-xs font-semibold tracking-tight text-white line-clamp-2 leading-snug group-hover:text-indigo-200 transition">
						{inc.title}
					</h3>

					<!-- Status Row -->
					<div class="flex items-center justify-between pt-1 border-t border-white/[0.04] text-[10px] font-mono">
						<div class="flex items-center gap-1.5">
							{#if inc.status === 'investigating'}
								<span class="relative flex h-2 w-2">
									<span class="animate-ping absolute inline-flex h-full w-full rounded-full bg-indigo-400 opacity-75"></span>
									<span class="relative inline-flex rounded-full h-2 w-2 bg-indigo-500"></span>
								</span>
								<span class="text-indigo-400 font-semibold capitalize">Investigating</span>
							{:else if inc.status === 'open'}
								<span class="w-2 h-2 rounded-full bg-rose-500"></span>
								<span class="text-rose-400 font-semibold capitalize">Open</span>
							{:else if inc.status === 'mitigated'}
								<span class="w-2 h-2 rounded-full bg-amber-400"></span>
								<span class="text-amber-400 font-semibold capitalize">Mitigated</span>
							{:else}
								<span class="w-2 h-2 rounded-full bg-emerald-500"></span>
								<span class="text-emerald-400 font-semibold capitalize">Resolved</span>
							{/if}
						</div>

						<span class="text-slate-600">
							#{inc.id.slice(0, 8)}
						</span>
					</div>
				</button>
			{/each}
		{/if}
	</div>
</div>
