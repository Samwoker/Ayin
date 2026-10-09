<script lang="ts">
	import type { Incident } from '../api/types';
	import { Card } from '#lib/components/ui/card/index.js';
	import { Badge, type BadgeVariant } from '#lib/components/ui/badge/index.js';
	import { Input } from '#lib/components/ui/input/index.js';

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

	function formatTimeAgo(isoString: string): string {
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

<Card class="p-4 border-slate-800 bg-slate-900/60 backdrop-blur-sm space-y-4">
	<!-- Header & Filters -->
	<div class="space-y-3">
		<div class="flex items-center justify-between">
			<h2 class="text-sm font-bold text-white uppercase tracking-wider font-mono flex items-center gap-2">
				<span>Incidents Queue</span>
				<Badge variant="secondary" class="text-[10px] px-2 py-0">
					{filteredIncidents.length}
				</Badge>
			</h2>

			<!-- Status Filter Tabs -->
			<div class="flex items-center rounded-lg bg-slate-950 p-0.5 border border-slate-800 text-[11px] font-mono">
				<button
					onclick={() => (selectedStatusFilter = 'all')}
					class="px-2.5 py-1 rounded-md transition cursor-pointer {selectedStatusFilter === 'all' ? 'bg-slate-800 text-white font-medium shadow-sm' : 'text-slate-400 hover:text-slate-200'}"
				>
					All
				</button>
				<button
					onclick={() => (selectedStatusFilter = 'active')}
					class="px-2.5 py-1 rounded-md transition cursor-pointer {selectedStatusFilter === 'active' ? 'bg-slate-800 text-rose-400 font-medium shadow-sm' : 'text-slate-400 hover:text-slate-200'}"
				>
					Active
				</button>
				<button
					onclick={() => (selectedStatusFilter = 'resolved')}
					class="px-2.5 py-1 rounded-md transition cursor-pointer {selectedStatusFilter === 'resolved' ? 'bg-slate-800 text-emerald-400 font-medium shadow-sm' : 'text-slate-400 hover:text-slate-200'}"
				>
					Resolved
				</button>
			</div>
		</div>

		<!-- Search Input -->
		<div class="relative">
			<span class="absolute left-3 top-2.5 text-slate-500 text-xs">🔍</span>
			<Input
				placeholder="Search incidents, services..."
				bind:value={searchQuery}
				class="pl-8 bg-slate-950 border-slate-800 text-xs h-9"
			/>
		</div>

		<!-- Severity Pills -->
		<div class="flex items-center gap-1.5 overflow-x-auto text-[11px] font-mono">
			{#each ['all', 'sev1', 'sev2', 'sev3', 'sev4'] as sev}
				<button
					onclick={() => (selectedSeverity = sev)}
					class="px-2.5 py-0.5 rounded-md border transition cursor-pointer uppercase {selectedSeverity === sev ? 'bg-slate-800 border-slate-600 text-white font-bold' : 'bg-slate-950/60 border-slate-800/80 text-slate-400 hover:border-slate-700'}"
				>
					{sev}
				</button>
			{/each}
		</div>
	</div>

	<!-- Incident Cards List -->
	<div class="space-y-2.5 max-h-[640px] overflow-y-auto pr-1">
		{#if filteredIncidents.length === 0}
			<div class="p-8 text-center rounded-lg border border-dashed border-slate-800 text-slate-500 text-xs font-mono">
				No incidents matching filter criteria
			</div>
		{:else}
			{#each filteredIncidents as inc (inc.id)}
				{@const isSelected = selectedIncidentId === inc.id}
				<button
					type="button"
					onclick={() => onSelect(inc)}
					class="w-full text-left p-3.5 rounded-xl border transition cursor-pointer text-xs space-y-2 relative overflow-hidden group {isSelected ? 'bg-slate-800/90 border-red-500/80 shadow-md shadow-red-950/20' : 'bg-slate-950/70 border-slate-800/80 hover:bg-slate-900/60 hover:border-slate-700'}"
				>
					{#if isSelected}
						<div class="absolute left-0 top-0 bottom-0 w-1 bg-gradient-to-b from-red-500 to-rose-600"></div>
					{/if}

					<!-- Top Row: Severity & Status -->
					<div class="flex items-center justify-between gap-2">
						<div class="flex items-center gap-2">
							<!-- Severity Badge -->
							<Badge variant={getSeverityBadgeVariant(inc.severity)} class="uppercase font-mono text-[10px] px-2 py-0.5">
								{inc.severity}
							</Badge>

							<!-- Service Tag -->
							<span class="font-mono text-slate-300 bg-slate-900 px-2 py-0.5 rounded border border-slate-800 text-[10px]">
								{inc.service}
							</span>
						</div>

						<!-- Time Ago -->
						<span class="text-[10px] text-slate-500 font-mono">
							{formatTimeAgo(inc.created_at || inc.opened_at || '')}
						</span>
					</div>

					<!-- Title -->
					<h3 class="font-medium text-slate-100 line-clamp-2 leading-snug group-hover:text-white transition">
						{inc.title}
					</h3>

					<!-- Bottom Row: Status Indicator -->
					<div class="flex items-center justify-between pt-1 text-[11px] font-mono text-slate-400">
						<div class="flex items-center gap-1.5">
							{#if inc.status === 'investigating'}
								<span class="relative flex h-2 w-2">
									<span class="animate-ping absolute inline-flex h-full w-full rounded-full bg-indigo-400 opacity-75"></span>
									<span class="relative inline-flex rounded-full h-2 w-2 bg-indigo-500"></span>
								</span>
								<span class="text-indigo-400 capitalize">Investigating</span>
							{:else if inc.status === 'open'}
								<span class="w-2 h-2 rounded-full bg-rose-500 inline-block"></span>
								<span class="text-rose-400 capitalize">Open</span>
							{:else if inc.status === 'mitigated'}
								<span class="w-2 h-2 rounded-full bg-amber-500 inline-block"></span>
								<span class="text-amber-400 capitalize">Mitigated</span>
							{:else}
								<span class="w-2 h-2 rounded-full bg-emerald-500 inline-block"></span>
								<span class="text-emerald-400 capitalize">Resolved</span>
							{/if}
						</div>

						<span class="text-[10px] text-slate-500">ID: {inc.id.slice(0, 8)}</span>
					</div>
				</button>
			{/each}
		{/if}
	</div>
</Card>
