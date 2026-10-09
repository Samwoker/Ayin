<script lang="ts">
	import type { HTMLAttributes } from 'svelte/elements';
	import type { Snippet } from 'svelte';
	import { cn } from '#lib/utils.js';

	let {
		class: className = '',
		children,
		...restProps
	}: HTMLAttributes<HTMLDivElement> & {
		children?: Snippet;
	} = $props();
</script>

<div
	class={cn(
		'rounded-lg border border-slate-800 bg-slate-900/80 text-slate-100 shadow-sm backdrop-blur',
		className
	)}
	{...restProps}
>
	{@render children?.()}
</div>
