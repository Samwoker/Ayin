<script lang="ts">
	import type { HTMLInputAttributes } from 'svelte/elements';
	import { cn } from '#lib/utils.js';

	let {
		class: className = '',
		value = $bindable(),
		type = 'text',
		...restProps
	}: HTMLInputAttributes = $props();
</script>

<input
	{type}
	bind:value
	class={cn(
		'flex h-9 w-full rounded-md border border-slate-700 bg-slate-900/80 px-3 py-1 text-sm text-slate-100 shadow-sm transition-colors placeholder:text-slate-500 focus-visible:border-emerald-500 focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-emerald-500 disabled:cursor-not-allowed disabled:opacity-50',
		className
	)}
	{...restProps}
/>
