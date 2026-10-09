<script lang="ts">
	import type { HTMLAttributes } from 'svelte/elements';
	import type { Snippet } from 'svelte';
	import { cn } from '#lib/utils.js';

	let {
		class: className = '',
		children,
		...restProps
	}: HTMLAttributes<HTMLParagraphElement> & {
		children?: Snippet;
	} = $props();
</script>

<p
	class={cn('text-xs text-slate-400', className)}
	{...restProps}
>
	{@render children?.()}
</p>
