<script lang="ts">
	import { Tabs } from 'bits-ui';
	import type { Snippet } from 'svelte';
	import { cn } from '#lib/utils.js';

	let {
		ref = $bindable(null),
		class: className = '',
		children,
		...restProps
	}: Tabs.TriggerProps & { children?: Snippet } = $props();
</script>

<Tabs.Trigger
	bind:ref
	class={cn(
		'inline-flex items-center justify-center whitespace-nowrap rounded-md px-3 py-1.5 text-xs font-medium ring-offset-background transition-all focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-emerald-500 focus-visible:ring-offset-2 disabled:pointer-events-none disabled:opacity-50 data-[state=active]:bg-slate-800 data-[state=active]:text-white data-[state=active]:shadow-sm cursor-pointer hover:text-slate-200',
		className
	)}
	{...restProps}
>
	{@render children?.()}
</Tabs.Trigger>
