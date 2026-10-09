<script lang="ts">
	import type { Runbook } from '../api/types';
	import { Card } from '#lib/components/ui/card/index.js';
	import { Badge } from '#lib/components/ui/badge/index.js';
	import { Button } from '#lib/components/ui/button/index.js';
	import { Separator } from '#lib/components/ui/separator/index.js';

	interface Props {
		runbooks: Runbook[];
		targetService?: string;
	}

	let { runbooks = [], targetService = '' }: Props = $props();

	let selectedRunbookId = $state<string | null>(null);

	let activeRunbook = $derived(
		runbooks.find((r) => r.id === selectedRunbookId) ||
			runbooks.find((r) => targetService && (r.service === targetService)) ||
			runbooks[0] ||
			null
	);

	let copiedSnippet = $state<string | null>(null);

	function copyToClipboard(text: string, id: string) {
		if (navigator?.clipboard) {
			navigator.clipboard.writeText(text);
			copiedSnippet = id;
			setTimeout(() => {
				if (copiedSnippet === id) copiedSnippet = null;
			}, 2000);
		}
	}
</script>

<div class="space-y-4">
	<div class="flex items-center justify-between">
		<div>
			<h2 class="text-sm font-bold text-white uppercase tracking-wider font-mono">
				Operational Runbooks & SOPs
			</h2>
			<p class="text-xs text-slate-400">
				Standard operational guidelines and verified commands for service incident mitigation
			</p>
		</div>

		<!-- Selector Tabs if multiple runbooks -->
		<div class="flex items-center gap-1 bg-slate-950 p-1 rounded-lg border border-slate-800 text-xs font-mono">
			{#each runbooks as rb (rb.id)}
				<button
					onclick={() => (selectedRunbookId = rb.id)}
					class="px-2.5 py-1 rounded transition cursor-pointer {activeRunbook?.id === rb.id ? 'bg-slate-800 text-white font-bold' : 'text-slate-400 hover:text-white'}"
				>
					{rb.service || rb.title.slice(0, 16)}
				</button>
			{/each}
		</div>
	</div>

	{#if activeRunbook}
		<Card class="p-6 space-y-5 border-slate-800 bg-slate-900/60 backdrop-blur-sm">
			<!-- Title & Service Header -->
			<div class="border-b border-slate-800/80 pb-4 flex items-start justify-between gap-4 flex-wrap">
				<div class="space-y-1">
					<div class="flex items-center gap-2">
						{#if activeRunbook.service}
							<Badge variant="secondary" class="bg-indigo-950 border-indigo-800/80 text-indigo-300 font-mono text-[10px] font-bold">
								{activeRunbook.service}
							</Badge>
						{/if}
						<h3 class="text-base font-bold text-white font-sans">
							{activeRunbook.title}
						</h3>
					</div>
					<p class="text-xs text-slate-400 font-sans">{activeRunbook.description}</p>
				</div>
			</div>

			<!-- Formatted Runbook Content -->
			<div class="max-w-none text-xs font-sans text-slate-300 space-y-4">
				<pre class="p-4 rounded-xl bg-slate-950 border border-slate-800 text-slate-200 font-mono text-xs whitespace-pre-wrap leading-relaxed overflow-x-auto">{activeRunbook.content_markdown || activeRunbook.markdown_content || ''}</pre>
			</div>

			<Separator class="bg-slate-800/60" />

			<!-- Quick Diagnostic Command Snippets -->
			<div class="space-y-2 pt-1">
				<span class="text-[11px] font-mono font-bold uppercase tracking-wider text-slate-400 block">
					Quick Copy Command:
				</span>
				<div class="flex items-center justify-between p-3 rounded-lg bg-slate-950 border border-slate-800 font-mono text-xs text-emerald-400">
					<code>kubectl rollout restart deployment/{activeRunbook.service || 'service'} -n prod</code>
					<Button
						variant="secondary"
						size="sm"
						onclick={() =>
							copyToClipboard(
								`kubectl rollout restart deployment/${activeRunbook.service || 'service'} -n prod`,
								'cmd-1'
							)}
						class="text-[11px] h-7"
					>
						{copiedSnippet === 'cmd-1' ? 'Copied! ✓' : 'Copy'}
					</Button>
				</div>
			</div>
		</Card>
	{:else}
		<div class="p-8 text-center rounded-xl border border-dashed border-slate-800 text-slate-500 text-xs font-mono">
			No runbooks registered for this service.
		</div>
	{/if}
</div>
