<script lang="ts">
	import { Tabs } from 'bits-ui';
	import type { Snippet } from 'svelte';
	import { cn } from '#lib/utils.js';

	let {
		ref = $bindable(null),
		class: className = '',
		children,
		...restProps
	}: Tabs.ContentProps & { children?: Snippet } = $props();
</script>

<Tabs.Content
	bind:ref
	class={cn(
		'mt-2 ring-offset-background focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-emerald-500 focus-visible:ring-offset-2',
		className
	)}
	{...restProps}
>
	{@render children?.()}
</Tabs.Content>
