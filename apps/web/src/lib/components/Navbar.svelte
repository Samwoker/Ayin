<script lang="ts">
	import { Badge } from '#lib/components/ui/badge/index.js';
	import { Button } from '#lib/components/ui/button/index.js';
	import { Radio, Zap, RefreshCw, Server, Activity, Sparkles, CreditCard, ShieldCheck, Eye, BookOpen } from '@lucide/svelte';

	interface Props {
		backendConnected: boolean;
		latencyMs: number;
		activeTenant: string;
		planTier?: string;
		runsUsed?: number;
		runsLimit?: number;
		onSimulate: () => void;
		onRefresh: () => void;
		onOpenPricing?: () => void;
		onOpenDocs?: () => void;
		isRefreshing: boolean;
	}

	let {
		backendConnected = false,
		latencyMs = 0,
		activeTenant = 'Acme Corp (Primary)',
		planTier = 'free',
		runsUsed = 0,
		runsLimit = 10,
		onSimulate,
		onRefresh,
		onOpenPricing,
		onOpenDocs,
		isRefreshing = false
	}: Props = $props();
</script>

<header class="border-b border-white/[0.08] bg-[#090a0d]/85 backdrop-blur-md sticky top-0 z-40 px-6 py-2.5 flex flex-wrap items-center justify-between gap-4 transition-all shadow-sm">
	<!-- Left: Brand & Status -->
	<div class="flex items-center space-x-3.5">
		<div class="relative w-9 h-9 rounded-xl bg-gradient-to-br from-indigo-500 via-violet-600 to-indigo-700 flex items-center justify-center font-mono font-bold text-white shadow-lg shadow-indigo-950/40 ring-1 ring-white/20">
			<Eye class="w-5 h-5 text-white" />
			<span class="absolute -bottom-0.5 -right-0.5 w-2.5 h-2.5 rounded-full bg-emerald-500 ring-2 ring-slate-950"></span>
		</div>
		<div>
			<div class="flex items-center gap-2">
				<h1 class="text-base font-bold tracking-tight text-white font-sans flex items-center gap-1.5">
					<span>Ayin</span>
					<span class="text-xs font-normal text-slate-400 font-mono tracking-normal">/ ዓይን</span>
				</h1>
				<Badge variant="sev1" class="text-[10px] px-2 py-0 animate-none font-mono">
					v0.1.0
				</Badge>
				<span class="hidden sm:inline-flex items-center gap-1 text-[10px] font-mono px-2 py-0.5 rounded-full bg-indigo-950/50 border border-indigo-700/40 text-indigo-300">
					<Sparkles class="w-3 h-3 text-indigo-400" />
					Gemini 2.5
				</span>
			</div>
			<p class="text-[11px] text-slate-400 font-sans">Autonomous Incident Response & Observability Eye</p>
		</div>
	</div>

	<!-- Center / Right: Organization & Real-time Connectivity -->
	<div class="flex items-center gap-3 text-xs">
		<!-- Tenant Org Selector -->
		<div class="flex items-center gap-2 px-3 py-1.5 rounded-lg bg-slate-900/90 border border-slate-800 text-slate-300 font-medium shadow-sm">
			<Server class="w-3.5 h-3.5 text-slate-400" />
			<span class="text-slate-400 font-mono text-[11px]">Org:</span>
			<span class="text-white font-semibold flex items-center gap-1.5 font-mono text-xs">
				<span class="w-1.5 h-1.5 rounded-full bg-blue-400"></span>
				{activeTenant}
			</span>
		</div>

		<!-- Plan Tier Pill / Pricing Trigger -->
		{#if planTier === 'pro'}
			<button
				onclick={onOpenPricing}
				class="flex items-center gap-1.5 px-3 py-1.5 rounded-lg border font-mono text-[11px] bg-indigo-950/60 border-indigo-700/70 text-indigo-200 hover:bg-indigo-900/60 hover:border-indigo-500 transition cursor-pointer shadow-sm shadow-indigo-950/30"
				title="Active Pro Tier. Click to manage subscription."
			>
				<Sparkles class="w-3.5 h-3.5 text-indigo-400" />
				<span class="font-bold">PRO</span>
				<span class="text-indigo-300/80 text-[10px]">({runsUsed}/{runsLimit} runs)</span>
			</button>
		{:else if planTier === 'enterprise'}
			<button
				onclick={onOpenPricing}
				class="flex items-center gap-1.5 px-3 py-1.5 rounded-lg border font-mono text-[11px] bg-emerald-950/60 border-emerald-700/70 text-emerald-200 hover:bg-emerald-900/60 transition cursor-pointer"
				title="Enterprise Tier. Click for details."
			>
				<ShieldCheck class="w-3.5 h-3.5 text-emerald-400" />
				<span class="font-bold">ENTERPRISE</span>
			</button>
		{:else}
			<button
				onclick={onOpenPricing}
				class="flex items-center gap-1.5 px-3 py-1.5 rounded-lg border font-mono text-[11px] bg-slate-900/90 border-slate-800 text-slate-300 hover:border-slate-700 hover:text-white transition cursor-pointer"
				title="Free Tier. Click to view plans or upgrade."
			>
				<Zap class="w-3.5 h-3.5 text-amber-400" />
				<span>FREE</span>
				<span class="text-slate-400 text-[10px]">({runsUsed}/{runsLimit} runs)</span>
			</button>

			<Button
				variant="outline"
				size="sm"
				onclick={onOpenPricing}
				class="border-indigo-500/50 bg-indigo-950/30 hover:bg-indigo-900/50 text-indigo-200 hover:text-white gap-1.5 shadow-sm active:scale-95 transition"
				title="Upgrade to Pro with Stripe"
			>
				<CreditCard class="w-3.5 h-3.5 text-indigo-400" />
				<span>Upgrade</span>
			</Button>
		{/if}

		<!-- Live Backend Connection Pill -->
		<div class="flex items-center gap-2 px-3 py-1.5 rounded-lg border font-mono text-[11px] {backendConnected ? 'bg-emerald-950/50 border-emerald-800/70 text-emerald-300 shadow-sm shadow-emerald-950/20' : 'bg-amber-950/50 border-amber-800/70 text-amber-300'}">
			{#if backendConnected}
				<span class="relative flex h-2 w-2">
					<span class="animate-ping absolute inline-flex h-full w-full rounded-full bg-emerald-400 opacity-75"></span>
					<span class="relative inline-flex rounded-full h-2 w-2 bg-emerald-500"></span>
				</span>
				<Activity class="w-3.5 h-3.5 text-emerald-400" />
				<span>Live API ({latencyMs}ms)</span>
			{:else}
				<Radio class="w-3.5 h-3.5 text-amber-400" />
				<span>Standalone / Mock</span>
			{/if}
		</div>

		<!-- Developer Integration Docs Action -->
		<Button
			variant="outline"
			size="sm"
			onclick={onOpenDocs}
			class="border-slate-800 bg-slate-900/80 hover:bg-slate-800 text-slate-200 hover:text-white gap-1.5 shadow-sm active:scale-95 transition"
			title="Open developer integration guides & webhook snippets"
		>
			<BookOpen class="w-3.5 h-3.5 text-indigo-400" />
			<span>Docs</span>
		</Button>

		<!-- Action: Trigger Fault Simulation -->
		<Button
			variant="destructive"
			size="sm"
			onclick={onSimulate}
			class="bg-red-950/80 hover:bg-red-900 border border-red-800/80 text-red-200 gap-1.5 shadow-sm active:scale-95 transition"
			title="Inject a test alert into the triage pipeline"
		>
			<Zap class="w-3.5 h-3.5 text-rose-400" />
			<span>Simulate Alert</span>
		</Button>

		<!-- Refresh / Sync Action -->
		<Button
			variant="secondary"
			size="sm"
			onclick={onRefresh}
			disabled={isRefreshing}
			class="border border-slate-700/80 bg-slate-900/90 text-slate-200 hover:text-white gap-1.5 shadow-sm active:scale-95 transition"
			title="Refresh incidents and runs"
		>
			<RefreshCw class="w-3.5 h-3.5 {isRefreshing ? 'animate-spin' : ''}" />
			<span>{isRefreshing ? 'Syncing...' : 'Sync'}</span>
		</Button>
	</div>
</header>
