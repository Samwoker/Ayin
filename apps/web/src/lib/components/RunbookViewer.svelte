<script lang="ts">
	import type { Runbook } from '../api/types';

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
			<p class="text-xs text-neutral-400">
				Standard operational guidelines and verified commands for service incident mitigation
			</p>
		</div>

		<!-- Selector Tabs if multiple runbooks -->
		<div class="flex items-center gap-1 bg-neutral-950 p-1 rounded-lg border border-neutral-800 text-xs font-mono">
			{#each runbooks as rb (rb.id)}
				<button
					onclick={() => (selectedRunbookId = rb.id)}
					class="px-2.5 py-1 rounded transition cursor-pointer {activeRunbook?.id === rb.id ? 'bg-neutral-800 text-white font-bold' : 'text-neutral-400 hover:text-white'}"
				>
					{rb.service || rb.title.slice(0, 16)}
				</button>
			{/each}
		</div>
	</div>

	{#if activeRunbook}
		<div class="rounded-xl border border-neutral-800 bg-neutral-900/40 p-6 space-y-5 backdrop-blur-sm">
			<!-- Title & Service Header -->
			<div class="border-b border-neutral-800/80 pb-4 flex items-start justify-between gap-4 flex-wrap">
				<div class="space-y-1">
					<div class="flex items-center gap-2">
						{#if activeRunbook.service}
							<span class="px-2 py-0.5 rounded bg-indigo-950 border border-indigo-800/80 text-indigo-300 font-mono text-[10px] font-bold">
								{activeRunbook.service}
							</span>
						{/if}
						<h3 class="text-base font-bold text-white font-sans">
							{activeRunbook.title}
						</h3>
					</div>
					<p class="text-xs text-neutral-400 font-sans">{activeRunbook.description}</p>
				</div>
			</div>

			<!-- Formatted Runbook Content -->
			<div class="prose prose-invert max-w-none text-xs font-sans text-neutral-300 space-y-4">
				<pre class="p-4 rounded-xl bg-neutral-950 border border-neutral-800 text-neutral-200 font-mono text-xs whitespace-pre-wrap leading-relaxed overflow-x-auto">{activeRunbook.content_markdown || activeRunbook.markdown_content || ''}</pre>
			</div>

			<!-- Quick Diagnostic Command Snippets -->
			<div class="space-y-2 pt-2 border-t border-neutral-800/60">
				<span class="text-[11px] font-mono font-bold uppercase tracking-wider text-neutral-400 block">
					Quick Copy Command:
				</span>
				<div class="flex items-center justify-between p-3 rounded-lg bg-neutral-950 border border-neutral-800 font-mono text-xs text-emerald-400">
					<code>kubectl rollout restart deployment/{activeRunbook.service || 'service'} -n prod</code>
					<button
						onclick={() =>
							copyToClipboard(
								`kubectl rollout restart deployment/${activeRunbook.service || 'service'} -n prod`,
								'cmd-1'
							)}
						class="px-2.5 py-1 rounded bg-neutral-800 hover:bg-neutral-700 text-neutral-200 text-[11px] transition cursor-pointer"
					>
						{copiedSnippet === 'cmd-1' ? 'Copied! ✓' : 'Copy'}
					</button>
				</div>
			</div>
		</div>
	{:else}
		<div class="p-8 text-center rounded-xl border border-dashed border-neutral-800 text-neutral-500 text-xs font-mono">
			No runbooks registered for this service.
		</div>
	{/if}
</div>
