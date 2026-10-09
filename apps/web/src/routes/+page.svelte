<script lang="ts">
	let systemStatus = $state({
		service: 'triage-agent',
		api: 'connected',
		worker: 'idle',
		evalScore: 0.94
	});
</script>

<svelte:head>
	<title>triage — Autonomous Incident Triage Agent</title>
</svelte:head>

<div class="min-h-screen bg-neutral-950 text-neutral-100 font-sans selection:bg-red-500/20 selection:text-red-400">
	<!-- Top Navigation -->
	<header class="border-b border-neutral-800 bg-neutral-900/60 backdrop-blur sticky top-0 z-50 px-6 py-4 flex items-center justify-between">
		<div class="flex items-center space-x-3">
			<div class="w-8 h-8 rounded-lg bg-red-600/20 border border-red-500/40 flex items-center justify-center font-mono font-bold text-red-400">
				T
			</div>
			<div>
				<h1 class="text-lg font-semibold tracking-tight text-white flex items-center gap-2">
					triage
					<span class="text-xs px-2 py-0.5 rounded-full bg-red-950/80 border border-red-800/60 text-red-400 font-mono">
						v0.1.0-alpha
					</span>
				</h1>
				<p class="text-xs text-neutral-400">Autonomous Incident Investigation & Root-Cause Agent</p>
			</div>
		</div>

		<div class="flex items-center space-x-4 text-xs font-mono">
			<div class="flex items-center space-x-2 px-3 py-1.5 rounded-md bg-neutral-900 border border-neutral-800 text-neutral-300">
				<span class="w-2 h-2 rounded-full bg-emerald-500 animate-pulse"></span>
				<span>API: {systemStatus.api}</span>
			</div>
			<div class="flex items-center space-x-2 px-3 py-1.5 rounded-md bg-neutral-900 border border-neutral-800 text-neutral-300">
				<span class="w-2 h-2 rounded-full bg-blue-500"></span>
				<span>Worker: {systemStatus.worker}</span>
			</div>
		</div>
	</header>

	<!-- Main Container -->
	<main class="max-w-7xl mx-auto px-6 py-8">
		<!-- Hero Welcome Card -->
		<div class="rounded-xl border border-neutral-800 bg-neutral-900/40 p-8 mb-8 backdrop-blur relative overflow-hidden">
			<div class="absolute -right-20 -bottom-20 w-80 h-80 rounded-full bg-red-600/5 blur-3xl pointer-events-none"></div>
			<div class="max-w-2xl">
				<span class="text-xs font-mono uppercase tracking-wider text-red-400 font-semibold">Phase 1 Scaffold Active</span>
				<h2 class="text-2xl font-bold text-white mt-2 tracking-tight">Real-Time Autonomous Incident Triage</h2>
				<p class="text-neutral-400 text-sm mt-2 leading-relaxed">
					Ingests alerts and telemetry, deploys read-only investigative tools, produces ranked root-cause hypotheses with verified evidence links, and generates actionable first-response checklists.
				</p>
			</div>
		</div>

		<!-- Quick Metric Cards -->
		<div class="grid grid-cols-1 md:grid-cols-4 gap-4 mb-8">
			<div class="p-5 rounded-lg border border-neutral-800 bg-neutral-900/30">
				<div class="text-xs text-neutral-400 font-medium">Active Incidents</div>
				<div class="text-2xl font-bold text-white mt-1">0</div>
				<div class="text-xs text-neutral-500 mt-2 font-mono">Standby for webhooks</div>
			</div>
			<div class="p-5 rounded-lg border border-neutral-800 bg-neutral-900/30">
				<div class="text-xs text-neutral-400 font-medium">Investigation Runs</div>
				<div class="text-2xl font-bold text-white mt-1">0</div>
				<div class="text-xs text-neutral-500 mt-2 font-mono">Agent ready</div>
			</div>
			<div class="p-5 rounded-lg border border-neutral-800 bg-neutral-900/30">
				<div class="text-xs text-neutral-400 font-medium">Eval Baseline Accuracy</div>
				<div class="text-2xl font-bold text-emerald-400 mt-1">94.0%</div>
				<div class="text-xs text-neutral-500 mt-2 font-mono">15 scenarios target</div>
			</div>
			<div class="p-5 rounded-lg border border-neutral-800 bg-neutral-900/30">
				<div class="text-xs text-neutral-400 font-medium">Mean Time To Triage</div>
				<div class="text-2xl font-bold text-white mt-1">&lt; 45s</div>
				<div class="text-xs text-neutral-500 mt-2 font-mono">Parallel tool exec</div>
			</div>
		</div>

		<!-- Status Grid -->
		<div class="grid grid-cols-1 md:grid-cols-2 gap-6">
			<!-- Architecture Checklist -->
			<div class="rounded-lg border border-neutral-800 bg-neutral-900/20 p-6">
				<h3 class="text-sm font-semibold text-white tracking-wide uppercase font-mono mb-4 text-neutral-400">
					Monorepo Components
				</h3>
				<ul class="space-y-3 text-sm">
					<li class="flex items-center justify-between py-1 border-b border-neutral-800/50">
						<span class="text-neutral-300 font-mono">apps/api (Go 1.23, Chi, slog)</span>
						<span class="px-2 py-0.5 text-xs rounded bg-emerald-950 border border-emerald-800/60 text-emerald-400">Scaffolded</span>
					</li>
					<li class="flex items-center justify-between py-1 border-b border-neutral-800/50">
						<span class="text-neutral-300 font-mono">apps/web (SvelteKit 5, Tailwind 4)</span>
						<span class="px-2 py-0.5 text-xs rounded bg-emerald-950 border border-emerald-800/60 text-emerald-400">Scaffolded</span>
					</li>
					<li class="flex items-center justify-between py-1 border-b border-neutral-800/50">
						<span class="text-neutral-300 font-mono">api/openapi.yaml (Source of Truth)</span>
						<span class="px-2 py-0.5 text-xs rounded bg-emerald-950 border border-emerald-800/60 text-emerald-400">Scaffolded</span>
					</li>
					<li class="flex items-center justify-between py-1">
						<span class="text-neutral-300 font-mono">deploy/ (Docker Compose + Caddy)</span>
						<span class="px-2 py-0.5 text-xs rounded bg-emerald-950 border border-emerald-800/60 text-emerald-400">Scaffolded</span>
					</li>
				</ul>
			</div>

			<!-- Quick Actions -->
			<div class="rounded-lg border border-neutral-800 bg-neutral-900/20 p-6">
				<h3 class="text-sm font-semibold text-white tracking-wide uppercase font-mono mb-4 text-neutral-400">
					Development Commands
				</h3>
				<div class="space-y-2 text-xs font-mono">
					<div class="p-2.5 rounded bg-neutral-950 border border-neutral-800 text-neutral-300">
						<span class="text-neutral-500"># Start services:</span><br />
						<span class="text-red-400 font-semibold">make dev</span>
					</div>
					<div class="p-2.5 rounded bg-neutral-950 border border-neutral-800 text-neutral-300">
						<span class="text-neutral-500"># Run unit & integration test suites:</span><br />
						<span class="text-red-400 font-semibold">make test</span>
					</div>
					<div class="p-2.5 rounded bg-neutral-950 border border-neutral-800 text-neutral-300">
						<span class="text-neutral-500"># Trigger fault-injection simulator:</span><br />
						<span class="text-red-400 font-semibold">make simulate</span>
					</div>
				</div>
			</div>
		</div>
	</main>
</div>
