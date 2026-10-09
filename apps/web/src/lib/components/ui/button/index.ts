import { cva, type VariantProps } from 'class-variance-authority';
export { default as Button } from './button.svelte';

export const buttonVariants = cva(
	'inline-flex items-center justify-center gap-2 whitespace-nowrap rounded-md text-sm font-medium transition-all focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-emerald-500/50 disabled:pointer-events-none disabled:opacity-50 select-none cursor-pointer',
	{
		variants: {
			variant: {
				default:
					'bg-emerald-600 text-white shadow hover:bg-emerald-500 active:bg-emerald-700 font-semibold',
				destructive:
					'bg-red-600/90 text-white shadow-sm hover:bg-red-500 active:bg-red-700',
				outline:
					'border border-slate-700 bg-slate-900/60 text-slate-200 hover:bg-slate-800 hover:text-white',
				secondary:
					'bg-slate-800 text-slate-200 shadow-sm hover:bg-slate-700 hover:text-white',
				ghost: 'text-slate-300 hover:bg-slate-800/80 hover:text-white',
				link: 'text-emerald-400 underline-offset-4 hover:underline'
			},
			size: {
				default: 'h-9 px-4 py-2',
				sm: 'h-7 rounded px-2.5 text-xs',
				lg: 'h-10 rounded-md px-6 text-base',
				icon: 'h-8 w-8 rounded-md p-0'
			}
		},
		defaultVariants: {
			variant: 'default',
			size: 'default'
		}
	}
);

export type ButtonVariant = VariantProps<typeof buttonVariants>['variant'];
export type ButtonSize = VariantProps<typeof buttonVariants>['size'];
