<script lang="ts">
	import { Dialog } from 'bits-ui';
	import type { Snippet } from 'svelte';
	import { cn } from '#lib/utils.js';

	let {
		class: className = '',
		children,
		...restProps
	}: Dialog.OverlayProps & { children?: Snippet } = $props();
</script>

<Dialog.Overlay
	class={cn(
		'fixed inset-0 z-50 bg-black/80 backdrop-blur-sm transition-all duration-150',
		className
	)}
	{...restProps}
>
	{@render children?.()}
</Dialog.Overlay>
