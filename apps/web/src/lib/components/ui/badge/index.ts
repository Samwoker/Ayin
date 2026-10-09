import { cva, type VariantProps } from 'class-variance-authority';
export { default as Badge } from './badge.svelte';

export const badgeVariants = cva(
	'inline-flex items-center gap-1.5 rounded-full border px-2.5 py-0.5 text-xs font-semibold transition-colors focus:outline-none focus:ring-2 focus:ring-emerald-500/50',
	{
		variants: {
			variant: {
				default:
					'border-emerald-500/30 bg-emerald-500/10 text-emerald-400',
				secondary:
					'border-slate-700 bg-slate-800/80 text-slate-300',
				destructive:
					'border-red-500/30 bg-red-500/10 text-red-400',
				warning:
					'border-amber-500/30 bg-amber-500/10 text-amber-400',
				outline:
					'border-slate-700 text-slate-300 bg-transparent',
				sev1:
					'border-red-500/40 bg-red-500/15 text-red-300 font-bold animate-pulse',
				sev2:
					'border-amber-500/40 bg-amber-500/15 text-amber-300 font-semibold',
				sev3:
					'border-sky-500/40 bg-sky-500/15 text-sky-300 font-medium',
				sev4:
					'border-slate-600/40 bg-slate-700/30 text-slate-400 font-normal'
			}
		},
		defaultVariants: {
			variant: 'default'
		}
	}
);

export type BadgeVariant = VariantProps<typeof badgeVariants>['variant'];
