<script lang="ts">
	import type { HTMLAttributes } from 'svelte/elements';
	import type { Snippet } from 'svelte';
	import { type BadgeVariant, badgeVariants } from './index.js';
	import { cn } from '#lib/utils.js';

	let {
		class: className = '',
		variant = 'default',
		children,
		...restProps
	}: HTMLAttributes<HTMLDivElement> & {
		variant?: BadgeVariant;
		children?: Snippet;
	} = $props();
</script>

<div
	class={cn(badgeVariants({ variant }), className)}
	{...restProps}
>
	{@render children?.()}
</div>
