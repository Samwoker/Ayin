<script lang="ts">
	import { Tabs } from 'bits-ui';
	import type { Snippet } from 'svelte';
	import { cn } from '#lib/utils.js';

	let {
		ref = $bindable(null),
		value = $bindable(),
		class: className = '',
		children,
		...restProps
	}: Tabs.RootProps & { children?: Snippet } = $props();
</script>

<Tabs.Root
	bind:ref
	bind:value
	class={cn('w-full', className)}
	{...restProps}
>
	{@render children?.()}
</Tabs.Root>
