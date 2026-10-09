<script lang="ts">
	import { Dialog } from 'bits-ui';
	import type { Snippet } from 'svelte';
	import { cn } from '#lib/utils.js';

	let {
		class: className = '',
		children,
		...restProps
	}: Dialog.DescriptionProps & { children?: Snippet } = $props();
</script>

<Dialog.Description
	class={cn('text-sm text-slate-400', className)}
	{...restProps}
>
	{@render children?.()}
</Dialog.Description>
