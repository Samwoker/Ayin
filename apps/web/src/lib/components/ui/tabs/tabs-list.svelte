<script lang="ts">
	import { Tabs } from 'bits-ui';
	import type { Snippet } from 'svelte';
	import { cn } from '#lib/utils.js';

	let {
		ref = $bindable(null),
		class: className = '',
		children,
		...restProps
	}: Tabs.ListProps & { children?: Snippet } = $props();
</script>

<Tabs.List
	bind:ref
	class={cn(
		'inline-flex h-9 items-center justify-center rounded-lg bg-slate-900/90 p-1 text-slate-400 border border-slate-800/80',
		className
	)}
	{...restProps}
>
	{@render children?.()}
</Tabs.List>
