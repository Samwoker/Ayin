<script lang="ts">
	import type { HTMLTextareaAttributes } from 'svelte/elements';
	import { cn } from '#lib/utils.js';

	let {
		class: className = '',
		value = $bindable(),
		...restProps
	}: HTMLTextareaAttributes = $props();
</script>

<textarea
	bind:value
	class={cn(
		'flex min-h-[60px] w-full rounded-md border border-slate-700 bg-slate-900/80 px-3 py-2 text-sm text-slate-100 shadow-sm placeholder:text-slate-500 focus-visible:border-emerald-500 focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-emerald-500 disabled:cursor-not-allowed disabled:opacity-50',
		className
	)}
	{...restProps}
></textarea>
