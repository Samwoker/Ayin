<script lang="ts">
	import { Separator } from 'bits-ui';
	import { cn } from '#lib/utils.js';

	let {
		ref = $bindable(null),
		class: className = '',
		orientation = 'horizontal',
		decorative = true,
		...restProps
	}: Separator.RootProps = $props();
</script>

<Separator.Root
	bind:ref
	{decorative}
	{orientation}
	class={cn(
		'shrink-0 bg-slate-800',
		orientation === 'horizontal' ? 'h-[1px] w-full' : 'h-full w-[1px]',
		className
	)}
	{...restProps}
/>
