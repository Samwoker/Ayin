<script lang="ts">
	import { Dialog } from 'bits-ui';
	import type { Snippet } from 'svelte';
	import { cn } from '#lib/utils.js';

	let {
		class: className = '',
		children,
		...restProps
	}: Dialog.TitleProps & { children?: Snippet } = $props();
</script>

<Dialog.Title
	class={cn('text-lg font-semibold leading-none tracking-tight text-white', className)}
	{...restProps}
>
	{@render children?.()}
</Dialog.Title>
