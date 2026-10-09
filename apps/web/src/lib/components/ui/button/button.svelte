<script lang="ts">
	import type { HTMLButtonAttributes } from 'svelte/elements';
	import type { Snippet } from 'svelte';
	import { type ButtonVariant, type ButtonSize, buttonVariants } from './index.js';
	import { cn } from '#lib/utils.js';

	let {
		class: className = '',
		variant = 'default',
		size = 'default',
		type = 'button',
		children,
		...restProps
	}: HTMLButtonAttributes & {
		variant?: ButtonVariant;
		size?: ButtonSize;
		children?: Snippet;
	} = $props();
</script>

<button
	{type}
	class={cn(buttonVariants({ variant, size }), className)}
	{...restProps}
>
	{@render children?.()}
</button>
