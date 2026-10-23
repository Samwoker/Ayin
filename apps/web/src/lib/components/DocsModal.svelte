<script lang="ts">
	import {
		Dialog,
		DialogContent,
		DialogDescription,
		DialogHeader,
		DialogTitle
	} from '#lib/components/ui/dialog/index.js';
	import { Button } from '#lib/components/ui/button/index.js';
	import { Badge } from '#lib/components/ui/badge/index.js';
	import {
		BookOpen,
		Copy,
		Check,
		GitBranch,
		Terminal,
		Bot,
		Webhook,
		ShieldCheck,
		ExternalLink
	} from '@lucide/svelte';

	interface Props {
		isOpen: boolean;
		onClose: () => void;
	}

	let { isOpen = false, onClose }: Props = $props();

	type DocTab = 'github' | 'gitlab' | 'webhooks' | 'mcp' | 'api';
	let activeTab = $state<DocTab>('github');
	let copiedKey = $state<string | null>(null);

	function copyToClipboard(key: string, text: string) {
		navigator.clipboard.writeText(text);
		copiedKey = key;
		setTimeout(() => {
			if (copiedKey === key) copiedKey = null;
		}, 2000);
	}

	const githubSnippet = `# .github/workflows/deploy.yml
name: Deploy Production
on:
  push:
    branches: [main]

jobs:
  deploy:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4

      - name: Deploy Service
        run: ./scripts/deploy-prod.sh

      # Notify Ayin so it can correlate future incidents with this commit
      - name: Register Deploy with Ayin (ዓይን)
        if: success()
        run: |
          curl -s -X POST "http://localhost:8080/api/v1/ingest/webhooks/generic" \\
            -H "Content-Type: application/json" \\
            -d '{
              "title": "Production Deployment: \${{ github.repository }}",
              "service": "\${{ github.event.repository.name }}",
              "severity": "info",
              "status": "resolved",
              "description": "Commit \${{ github.sha }} deployed by \${{ github.actor }}",
              "labels": {
                "git_commit": "\${{ github.sha }}",
                "git_branch": "\${{ github.ref_name }}",
                "version": "\${{ github.ref_name }}"
              }
            }'`;

	const gitlabSnippet = `# .gitlab-ci.yml
deploy_production:
  stage: deploy
  script:
    - ./scripts/deploy-prod.sh
    # Notify Ayin with commit hash and deployer
    - |
      curl -s -X POST "http://localhost:8080/api/v1/ingest/webhooks/generic" \\
        -H "Content-Type: application/json" \\
        -d "{
          \\"title\\": \\"Production Deployment: $CI_PROJECT_NAME\\",
          \\"service\\": \\"$CI_PROJECT_NAME\\",
          \\"severity\\": \\"info\\",
          \\"status\\": \\"resolved\\",
          \\"description\\": \\"Commit $CI_COMMIT_SHA deployed by $GITLAB_USER_LOGIN\\",
          \\"labels\\": {
            \\"git_commit\\": \\"$CI_COMMIT_SHA\\",
            \\"git_branch\\": \\"$CI_COMMIT_REF_NAME\\"
          }
        }"
  only:
    - main`;

	const alertmanagerSnippet = `# alertmanager.yml
receivers:
  - name: 'ayin-receiver'
    webhook_configs:
      - url: 'http://localhost:8080/api/v1/ingest/webhooks/alertmanager'
        send_resolved: true
        http_config:
          # Optional HMAC shared secret header
          bearer_token: 'acme_webhook_secret_32bytes_hex_123'

route:
  receiver: 'ayin-receiver'
  group_by: ['alertname', 'service']
  group_wait: 10s
  repeat_interval: 1h`;

	const mcpSnippet = `// Cursor or Claude Desktop config (~/.cursor/mcp.json or claude_desktop_config.json)
{
  "mcpServers": {
    "ayin": {
      "command": "/home/seraph/Desktop/side/Ayin/apps/api/bin/mcp",
      "args": [
        "-transport", "stdio",
        "-org-id", "00000000-0000-0000-0000-000000000001"
      ]
    }
  }
}`;

	const apiSnippet = `# 1. Trigger an alert directly
curl -X POST http://localhost:8080/api/v1/ingest/webhooks/generic \\
  -H "Content-Type: application/json" \\
  -d '{
    "title": "500 Error Surge on Checkout",
    "service": "checkout-worker",
    "severity": "sev1",
    "status": "firing",
    "description": "Unhandled database lock deadlock"
  }'

# 2. Query active incidents with the Ayin CLI
./apps/api/bin/triage incidents list --severity=sev1`;
</script>

<Dialog open={isOpen} onOpenChange={(open) => !open && onClose()}>
	<DialogContent class="max-w-3xl bg-slate-950 border border-slate-800 text-slate-100 p-0 overflow-hidden shadow-2xl rounded-2xl">
		<!-- Header -->
		<div class="p-6 border-b border-slate-800/80 bg-slate-900/50">
			<div class="flex items-center justify-between gap-4">
				<div class="flex items-center gap-3">
					<div class="w-10 h-10 rounded-xl bg-indigo-500/10 border border-indigo-500/20 flex items-center justify-center text-indigo-400">
						<BookOpen class="w-5 h-5" />
					</div>
					<div>
						<DialogTitle class="text-lg font-bold text-white flex items-center gap-2">
							Developer Integration Guide
							<Badge variant="secondary" class="font-mono text-[10px] text-indigo-300 bg-indigo-950/60 border-indigo-800/50">
								Ayin 👁️ API
							</Badge>
						</DialogTitle>
						<DialogDescription class="text-xs text-slate-400">
							Connect your repositories, CI/CD pipelines, and monitoring tools to Ayin in minutes.
						</DialogDescription>
					</div>
				</div>
				<div class="flex items-center gap-2">
					<Badge variant="outline" class="font-mono text-[11px] text-emerald-400 border-emerald-800/50 bg-emerald-950/30">
						<ShieldCheck class="w-3 h-3 mr-1" />
						Read-Only Diagnostics
					</Badge>
				</div>
			</div>

			<!-- Navigation Tabs -->
			<div class="flex items-center gap-2 mt-6 overflow-x-auto pb-1 text-xs font-mono">
				<button
					onclick={() => (activeTab = 'github')}
					class="px-3.5 py-1.5 rounded-lg border transition-all flex items-center gap-2 {activeTab === 'github' ? 'bg-indigo-600/20 border-indigo-500/50 text-indigo-200 font-semibold' : 'bg-slate-900/50 border-slate-800 text-slate-400 hover:text-slate-200'}"
				>
					<GitBranch class="w-3.5 h-3.5" />
					GitHub Actions
				</button>
				<button
					onclick={() => (activeTab = 'gitlab')}
					class="px-3.5 py-1.5 rounded-lg border transition-all flex items-center gap-2 {activeTab === 'gitlab' ? 'bg-indigo-600/20 border-indigo-500/50 text-indigo-200 font-semibold' : 'bg-slate-900/50 border-slate-800 text-slate-400 hover:text-slate-200'}"
				>
					<Terminal class="w-3.5 h-3.5" />
					GitLab CI/CD
				</button>
				<button
					onclick={() => (activeTab = 'webhooks')}
					class="px-3.5 py-1.5 rounded-lg border transition-all flex items-center gap-2 {activeTab === 'webhooks' ? 'bg-indigo-600/20 border-indigo-500/50 text-indigo-200 font-semibold' : 'bg-slate-900/50 border-slate-800 text-slate-400 hover:text-slate-200'}"
				>
					<Webhook class="w-3.5 h-3.5" />
					Alert Webhooks
				</button>
				<button
					onclick={() => (activeTab = 'mcp')}
					class="px-3.5 py-1.5 rounded-lg border transition-all flex items-center gap-2 {activeTab === 'mcp' ? 'bg-indigo-600/20 border-indigo-500/50 text-indigo-200 font-semibold' : 'bg-slate-900/50 border-slate-800 text-slate-400 hover:text-slate-200'}"
				>
					<Bot class="w-3.5 h-3.5" />
					Cursor / Claude (MCP)
				</button>
				<button
					onclick={() => (activeTab = 'api')}
					class="px-3.5 py-1.5 rounded-lg border transition-all flex items-center gap-2 {activeTab === 'api' ? 'bg-indigo-600/20 border-indigo-500/50 text-indigo-200 font-semibold' : 'bg-slate-900/50 border-slate-800 text-slate-400 hover:text-slate-200'}"
				>
					<Terminal class="w-3.5 h-3.5" />
					REST / CLI
				</button>
			</div>
		</div>

		<!-- Body -->
		<div class="p-6 space-y-4 max-h-[60vh] overflow-y-auto">
			{#if activeTab === 'github'}
				<div class="space-y-3">
					<div class="flex items-center justify-between">
						<div>
							<h3 class="text-sm font-semibold text-white">Track Deployments from GitHub Actions</h3>
							<p class="text-xs text-slate-400">
								Add this step at the end of your deploy workflow. When an alert triggers, Ayin checks this commit hash first.
							</p>
						</div>
						<Button
							variant="outline"
							size="sm"
							class="h-8 gap-1.5 text-xs font-mono"
							onclick={() => copyToClipboard('gh', githubSnippet)}
						>
							{#if copiedKey === 'gh'}
								<Check class="w-3.5 h-3.5 text-emerald-400" />
								<span>Copied</span>
							{:else}
								<Copy class="w-3.5 h-3.5" />
								<span>Copy YAML</span>
							{/if}
						</Button>
					</div>
					<pre class="p-4 rounded-xl bg-slate-900 border border-slate-800 text-slate-200 font-mono text-xs overflow-x-auto leading-relaxed">{githubSnippet}</pre>
				</div>
			{:else if activeTab === 'gitlab'}
				<div class="space-y-3">
					<div class="flex items-center justify-between">
						<div>
							<h3 class="text-sm font-semibold text-white">Track Deployments from GitLab CI/CD</h3>
							<p class="text-xs text-slate-400">
								Include this curl command in your production deploy stage to notify Ayin with the latest commit SHA.
							</p>
						</div>
						<Button
							variant="outline"
							size="sm"
							class="h-8 gap-1.5 text-xs font-mono"
							onclick={() => copyToClipboard('gl', gitlabSnippet)}
						>
							{#if copiedKey === 'gl'}
								<Check class="w-3.5 h-3.5 text-emerald-400" />
								<span>Copied</span>
							{:else}
								<Copy class="w-3.5 h-3.5" />
								<span>Copy YAML</span>
							{/if}
						</Button>
					</div>
					<pre class="p-4 rounded-xl bg-slate-900 border border-slate-800 text-slate-200 font-mono text-xs overflow-x-auto leading-relaxed">{gitlabSnippet}</pre>
				</div>
			{:else if activeTab === 'webhooks'}
				<div class="space-y-3">
					<div class="flex items-center justify-between">
						<div>
							<h3 class="text-sm font-semibold text-white">Forward Alerts from Prometheus / Alertmanager</h3>
							<p class="text-xs text-slate-400">
								Configure Prometheus Alertmanager or Grafana to send alerts directly into Ayin's ingestion queue.
							</p>
						</div>
						<Button
							variant="outline"
							size="sm"
							class="h-8 gap-1.5 text-xs font-mono"
							onclick={() => copyToClipboard('am', alertmanagerSnippet)}
						>
							{#if copiedKey === 'am'}
								<Check class="w-3.5 h-3.5 text-emerald-400" />
								<span>Copied</span>
							{:else}
								<Copy class="w-3.5 h-3.5" />
								<span>Copy YAML</span>
							{/if}
						</Button>
					</div>
					<pre class="p-4 rounded-xl bg-slate-900 border border-slate-800 text-slate-200 font-mono text-xs overflow-x-auto leading-relaxed">{alertmanagerSnippet}</pre>
				</div>
			{:else if activeTab === 'mcp'}
				<div class="space-y-3">
					<div class="flex items-center justify-between">
						<div>
							<h3 class="text-sm font-semibold text-white">Model Context Protocol (Cursor & Claude)</h3>
							<p class="text-xs text-slate-400">
								Connect your local coding assistant to Ayin's live diagnostic tools (<code class="text-indigo-300">log_search</code>, <code class="text-indigo-300">recent_deploys</code>, <code class="text-indigo-300">query_metrics</code>).
							</p>
						</div>
						<Button
							variant="outline"
							size="sm"
							class="h-8 gap-1.5 text-xs font-mono"
							onclick={() => copyToClipboard('mcp', mcpSnippet)}
						>
							{#if copiedKey === 'mcp'}
								<Check class="w-3.5 h-3.5 text-emerald-400" />
								<span>Copied</span>
							{:else}
								<Copy class="w-3.5 h-3.5" />
								<span>Copy JSON</span>
							{/if}
						</Button>
					</div>
					<pre class="p-4 rounded-xl bg-slate-900 border border-slate-800 text-slate-200 font-mono text-xs overflow-x-auto leading-relaxed">{mcpSnippet}</pre>
				</div>
			{:else if activeTab === 'api'}
				<div class="space-y-3">
					<div class="flex items-center justify-between">
						<div>
							<h3 class="text-sm font-semibold text-white">Manual Ingestion & Terminal CLI</h3>
							<p class="text-xs text-slate-400">
								Test the webhook API manually with curl or inspect incidents using the Ayin CLI binary.
							</p>
						</div>
						<Button
							variant="outline"
							size="sm"
							class="h-8 gap-1.5 text-xs font-mono"
							onclick={() => copyToClipboard('api', apiSnippet)}
						>
							{#if copiedKey === 'api'}
								<Check class="w-3.5 h-3.5 text-emerald-400" />
								<span>Copied</span>
							{:else}
								<Copy class="w-3.5 h-3.5" />
								<span>Copy Bash</span>
							{/if}
						</Button>
					</div>
					<pre class="p-4 rounded-xl bg-slate-900 border border-slate-800 text-slate-200 font-mono text-xs overflow-x-auto leading-relaxed">{apiSnippet}</pre>
				</div>
			{/if}
		</div>

		<!-- Footer -->
		<div class="p-4 border-t border-slate-800/80 bg-slate-900/60 flex items-center justify-between text-xs text-slate-400">
			<div class="flex items-center gap-1.5">
				<span>Documentation & Architecture:</span>
				<a href="https://github.com/triage-agent/triage" target="_blank" class="text-indigo-400 hover:text-indigo-300 font-mono flex items-center gap-1">
					docs/architecture.md <ExternalLink class="w-3 h-3" />
				</a>
			</div>
			<Button variant="secondary" size="sm" onclick={onClose}>
				Close
			</Button>
		</div>
	</DialogContent>
</Dialog>
