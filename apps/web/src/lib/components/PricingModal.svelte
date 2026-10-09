<script lang="ts">
	import type { SubscriptionInfo } from '../api/types';
	import { apiClient } from '../api/client';
	import {
		Dialog,
		DialogContent,
		DialogDescription,
		DialogHeader,
		DialogTitle
	} from '#lib/components/ui/dialog/index.js';
	import { Button } from '#lib/components/ui/button/index.js';
	import { Badge } from '#lib/components/ui/badge/index.js';
	import { Card } from '#lib/components/ui/card/index.js';
	import {
		Check,
		Sparkles,
		ShieldCheck,
		CreditCard,
		ExternalLink,
		Zap,
		Activity,
		Loader2
	} from '@lucide/svelte';

	interface Props {
		isOpen: boolean;
		onClose: () => void;
		subscription: SubscriptionInfo | null;
	}

	let { isOpen = false, onClose, subscription = null }: Props = $props();

	let isCheckingOut = $state(false);
	let isOpeningPortal = $state(false);
	let errorMessage = $state<string | null>(null);

	const activeTier = $derived(subscription?.plan_tier || 'free');
	const runsUsed = $derived(subscription?.monthly_runs_used ?? 0);
	const runsLimit = $derived(subscription?.monthly_runs_limit ?? 10);
	const runsRemaining = $derived(Math.max(0, runsLimit - runsUsed));
	const quotaPercent = $derived(
		Math.min(100, Math.round((runsUsed / Math.max(1, runsLimit)) * 100))
	);

	function handleOpenChange(open: boolean) {
		if (!open) {
			errorMessage = null;
			onClose();
		}
	}

	async function handleCheckout() {
		try {
			isCheckingOut = true;
			errorMessage = null;
			const res = await apiClient.createCheckoutSession({
				success_url: window.location.origin + '?upgraded=true',
				cancel_url: window.location.origin + '?canceled=true'
			});
			if (res.checkout_url) {
				window.location.href = res.checkout_url;
			}
		} catch (err: unknown) {
			errorMessage = err instanceof Error ? err.message : 'Failed to launch Stripe checkout';
		} finally {
			isCheckingOut = false;
		}
	}

	async function handleOpenPortal() {
		try {
			isOpeningPortal = true;
			errorMessage = null;
			const res = await apiClient.createPortalSession({
				return_url: window.location.origin
			});
			if (res.portal_url) {
				window.location.href = res.portal_url;
			}
		} catch (err: unknown) {
			errorMessage = err instanceof Error ? err.message : 'Failed to launch Stripe billing portal';
		} finally {
			isOpeningPortal = false;
		}
	}
</script>

<Dialog open={isOpen} onOpenChange={handleOpenChange}>
	<DialogContent class="max-w-4xl bg-slate-950 border-slate-800 text-slate-100 shadow-2xl p-6 sm:p-8">
		<DialogHeader class="space-y-2">
			<div class="flex items-center justify-between">
				<DialogTitle class="flex items-center gap-2.5 text-xl font-bold font-sans text-white">
					<CreditCard class="w-5 h-5 text-indigo-400" />
					<span>Plans & Subscription Management</span>
				</DialogTitle>
				<Badge variant="outline" class="font-mono text-xs border-indigo-500/40 text-indigo-300 bg-indigo-950/40 px-2.5 py-0.5">
					Stripe Billing Engine
				</Badge>
			</div>
			<DialogDescription class="text-xs text-slate-400 font-sans">
				Select an incident triage tier optimized for your organization. Powered by Google Gemini multi-agent reasoning.
			</DialogDescription>
		</DialogHeader>

		<!-- Usage & Quota Banner -->
		<div class="rounded-xl border border-slate-800 bg-slate-900/80 p-4 space-y-2.5">
			<div class="flex flex-wrap items-center justify-between gap-2 text-xs">
				<div class="flex items-center gap-2">
					<Activity class="w-4 h-4 text-emerald-400" />
					<span class="font-medium text-slate-200">Current Monthly Quota:</span>
					<span class="font-mono font-semibold text-white">
						{runsUsed} / {runsLimit} runs used
					</span>
					<span class="text-slate-400 font-mono text-[11px]">
						({runsRemaining} remaining)
					</span>
				</div>
				<div class="flex items-center gap-2">
					<span class="text-[11px] text-slate-400 font-mono">Current Tier:</span>
					<Badge
						variant={activeTier === 'pro' ? 'default' : activeTier === 'enterprise' ? 'sev1' : 'secondary'}
						class="uppercase font-mono text-[10px] tracking-wider px-2 py-0.5"
					>
						{activeTier}
					</Badge>
				</div>
			</div>

			<!-- Progress Bar -->
			<div class="w-full bg-slate-800/80 rounded-full h-2 overflow-hidden">
				<div
					class="h-full transition-all duration-500 {quotaPercent >= 90 ? 'bg-rose-500' : quotaPercent >= 70 ? 'bg-amber-500' : 'bg-emerald-500'}"
					style="width: {quotaPercent}%;"
				></div>
			</div>
		</div>

		{#if errorMessage}
			<div class="p-3 rounded-lg border border-red-800/60 bg-red-950/40 text-red-300 text-xs font-mono">
				{errorMessage}
			</div>
		{/if}

		<!-- Pricing Cards Grid -->
		<div class="grid grid-cols-1 md:grid-cols-3 gap-4 pt-2">
			<!-- Free Plan -->
			<Card class="p-5 flex flex-col justify-between border-slate-800 bg-slate-900/60 hover:border-slate-700 transition">
				<div class="space-y-4">
					<div class="space-y-1.5">
						<div class="flex items-center justify-between">
							<h3 class="text-base font-bold text-white">Free</h3>
							{#if activeTier === 'free'}
								<Badge variant="outline" class="text-[10px] border-emerald-500/50 text-emerald-300 bg-emerald-950/30">
									Current Plan
								</Badge>
							{/if}
						</div>
						<p class="text-[11px] text-slate-400 leading-snug">
							Essential triage automation for solo engineers and small teams.
						</p>
					</div>

					<div class="flex items-baseline gap-1">
						<span class="text-2xl font-bold font-mono text-white">$0</span>
						<span class="text-xs text-slate-400">/ month</span>
					</div>

					<div class="border-t border-slate-800/80 pt-3 space-y-2 text-xs text-slate-300">
						<div class="flex items-start gap-2">
							<Check class="w-3.5 h-3.5 text-emerald-400 mt-0.5 shrink-0" />
							<span><strong>10</strong> investigation runs / mo</span>
						</div>
						<div class="flex items-start gap-2">
							<Check class="w-3.5 h-3.5 text-emerald-400 mt-0.5 shrink-0" />
							<span><strong>Gemini 2.5 Flash</strong> analysis</span>
						</div>
						<div class="flex items-start gap-2">
							<Check class="w-3.5 h-3.5 text-emerald-400 mt-0.5 shrink-0" />
							<span><strong>10 req/min</strong> rate limit (burst 20)</span>
						</div>
						<div class="flex items-start gap-2">
							<Check class="w-3.5 h-3.5 text-emerald-400 mt-0.5 shrink-0" />
							<span>Standard runbook search</span>
						</div>
						<div class="flex items-start gap-2">
							<Check class="w-3.5 h-3.5 text-emerald-400 mt-0.5 shrink-0" />
							<span>Community support</span>
						</div>
					</div>
				</div>

				<div class="pt-5">
					<Button
						variant="outline"
						class="w-full text-xs border-slate-800 text-slate-400"
						disabled
					>
						{activeTier === 'free' ? 'Active Tier' : 'Included'}
					</Button>
				</div>
			</Card>

			<!-- Pro Plan (Featured) -->
			<Card class="p-5 flex flex-col justify-between border-indigo-500/60 bg-gradient-to-b from-indigo-950/40 via-slate-900/90 to-slate-900 shadow-xl shadow-indigo-950/30 relative">
				<div class="absolute -top-3 left-1/2 -translate-x-1/2">
					<Badge class="bg-indigo-600 hover:bg-indigo-600 text-white font-mono text-[10px] tracking-wider uppercase px-2.5 shadow-md">
						Most Popular
					</Badge>
				</div>

				<div class="space-y-4">
					<div class="space-y-1.5 pt-1">
						<div class="flex items-center justify-between">
							<h3 class="text-base font-bold text-white flex items-center gap-1.5">
								<span>Pro</span>
								<Sparkles class="w-4 h-4 text-indigo-400" />
							</h3>
							{#if activeTier === 'pro'}
								<Badge variant="outline" class="text-[10px] border-indigo-400/50 text-indigo-300 bg-indigo-950/50">
									Active Plan
								</Badge>
							{/if}
						</div>
						<p class="text-[11px] text-slate-300 leading-snug">
							Full automated triage & remediation engine for engineering teams.
						</p>
					</div>

					<div class="flex items-baseline gap-1">
						<span class="text-2xl font-bold font-mono text-white">$49</span>
						<span class="text-xs text-slate-400">/ month</span>
					</div>

					<div class="border-t border-slate-800/80 pt-3 space-y-2 text-xs text-slate-200">
						<div class="flex items-start gap-2">
							<Check class="w-3.5 h-3.5 text-indigo-400 mt-0.5 shrink-0" />
							<span><strong>250</strong> investigation runs / mo</span>
						</div>
						<div class="flex items-start gap-2">
							<Check class="w-3.5 h-3.5 text-indigo-400 mt-0.5 shrink-0" />
							<span><strong>Gemini 2.5 Pro & Flash</strong> engine</span>
						</div>
						<div class="flex items-start gap-2">
							<Check class="w-3.5 h-3.5 text-indigo-400 mt-0.5 shrink-0" />
							<span><strong>60 req/min</strong> rate limit (burst 100)</span>
						</div>
						<div class="flex items-start gap-2">
							<Check class="w-3.5 h-3.5 text-indigo-400 mt-0.5 shrink-0" />
							<span>Automated remediation proposals</span>
						</div>
						<div class="flex items-start gap-2">
							<Check class="w-3.5 h-3.5 text-indigo-400 mt-0.5 shrink-0" />
							<span>Real-time SSE event streaming</span>
						</div>
						<div class="flex items-start gap-2">
							<Check class="w-3.5 h-3.5 text-indigo-400 mt-0.5 shrink-0" />
							<span>Priority Slack & PagerDuty alerts</span>
						</div>
					</div>
				</div>

				<div class="pt-5 space-y-2">
					{#if activeTier === 'pro'}
						<Button
							variant="outline"
							class="w-full text-xs border-indigo-500/60 bg-indigo-950/30 text-indigo-200 hover:bg-indigo-900/50 gap-1.5"
							onclick={handleOpenPortal}
							disabled={isOpeningPortal}
						>
							{#if isOpeningPortal}
								<Loader2 class="w-3.5 h-3.5 animate-spin" />
								<span>Opening Portal...</span>
							{:else}
								<ExternalLink class="w-3.5 h-3.5" />
								<span>Manage in Stripe Portal</span>
							{/if}
						</Button>
					{:else}
						<Button
							class="w-full text-xs bg-indigo-600 hover:bg-indigo-500 text-white font-semibold gap-1.5 shadow-lg shadow-indigo-950/50"
							onclick={handleCheckout}
							disabled={isCheckingOut}
						>
							{#if isCheckingOut}
								<Loader2 class="w-3.5 h-3.5 animate-spin" />
								<span>Redirecting to Stripe...</span>
							{:else}
								<CreditCard class="w-3.5 h-3.5" />
								<span>Upgrade with Stripe</span>
							{/if}
						</Button>
					{/if}
				</div>
			</Card>

			<!-- Enterprise Plan -->
			<Card class="p-5 flex flex-col justify-between border-slate-800 bg-slate-900/60 hover:border-slate-700 transition">
				<div class="space-y-4">
					<div class="space-y-1.5">
						<div class="flex items-center justify-between">
							<h3 class="text-base font-bold text-white flex items-center gap-1.5">
								<span>Enterprise</span>
								<ShieldCheck class="w-4 h-4 text-emerald-400" />
							</h3>
							{#if activeTier === 'enterprise'}
								<Badge variant="outline" class="text-[10px] border-emerald-400/50 text-emerald-300 bg-emerald-950/40">
									Active
								</Badge>
							{/if}
						</div>
						<p class="text-[11px] text-slate-400 leading-snug">
							Dedicated reliability infrastructure for high-scale enterprise operations.
						</p>
					</div>

					<div class="flex items-baseline gap-1">
						<span class="text-2xl font-bold font-mono text-white">$299+</span>
						<span class="text-xs text-slate-400">/ month</span>
					</div>

					<div class="border-t border-slate-800/80 pt-3 space-y-2 text-xs text-slate-300">
						<div class="flex items-start gap-2">
							<Check class="w-3.5 h-3.5 text-emerald-400 mt-0.5 shrink-0" />
							<span><strong>2,500+</strong> investigation runs / mo</span>
						</div>
						<div class="flex items-start gap-2">
							<Check class="w-3.5 h-3.5 text-emerald-400 mt-0.5 shrink-0" />
							<span><strong>300 req/min</strong> rate limit (burst 500)</span>
						</div>
						<div class="flex items-start gap-2">
							<Check class="w-3.5 h-3.5 text-emerald-400 mt-0.5 shrink-0" />
							<span>Bring-Your-Own-Key (BYOK) Gemini</span>
						</div>
						<div class="flex items-start gap-2">
							<Check class="w-3.5 h-3.5 text-emerald-400 mt-0.5 shrink-0" />
							<span>Dedicated VPC deployment</span>
						</div>
						<div class="flex items-start gap-2">
							<Check class="w-3.5 h-3.5 text-emerald-400 mt-0.5 shrink-0" />
							<span>99.99% SLA & 24/7 on-call engineer</span>
						</div>
					</div>
				</div>

				<div class="pt-5">
					<Button
						variant="outline"
						class="w-full text-xs border-slate-700 bg-slate-800/80 text-slate-200 hover:text-white"
						onclick={() => window.open('mailto:enterprise@triage.internal?subject=Enterprise%20Tier%20Inquiry', '_blank')}
					>
						<span>Contact Enterprise</span>
					</Button>
				</div>
			</Card>
		</div>
	</DialogContent>
</Dialog>
