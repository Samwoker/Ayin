<script lang="ts">
	import { Dialog } from 'bits-ui';
	import type { Snippet } from 'svelte';
	import DialogOverlay from './dialog-overlay.svelte';
	import { X } from '@lucide/svelte';
	import { cn } from '#lib/utils.js';

	let {
		class: className = '',
		children,
		...restProps
	}: Dialog.ContentProps & { children?: Snippet } = $props();
</script>

<Dialog.Portal>
	<DialogOverlay />
	<Dialog.Content
		class={cn(
			'fixed left-[50%] top-[50%] z-50 grid w-full max-w-lg translate-x-[-50%] translate-y-[-50%] gap-4 border border-slate-800 bg-slate-900 p-6 shadow-2xl duration-200 sm:rounded-xl',
			className
		)}
		{...restProps}
	>
		{@render children?.()}
		<Dialog.Close
			class="absolute right-4 top-4 rounded-sm opacity-70 transition-opacity hover:opacity-100 focus:outline-none focus:ring-2 focus:ring-emerald-500 disabled:pointer-events-none text-slate-400 hover:text-white cursor-pointer"
		>
			<X class="h-4 w-4" />
			<span class="sr-only">Close</span>
		</Dialog.Close>
	</Dialog.Content>
</Dialog.Portal>
