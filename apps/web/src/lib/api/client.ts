import type { HealthResponse, ReadyResponse, StatusResponse, SystemOverview } from './types';

export class ApiClient {
	private baseUrl: string;
	private customFetch?: typeof fetch;

	constructor(baseUrl?: string, customFetch?: typeof fetch) {
		if (baseUrl) {
			this.baseUrl = baseUrl.replace(/\/+$/, '');
		} else if (typeof window !== 'undefined') {
			// In browser, allow relative calls or fall back to localhost:8080 if origin differs
			this.baseUrl = window.location.port === '8080' ? '' : 'http://localhost:8080';
		} else {
			// In server environment (SSR / Node)
			this.baseUrl = process.env.PUBLIC_API_URL || 'http://localhost:8080';
		}
		this.customFetch = customFetch;
	}

	private get fetcher(): typeof fetch {
		if (this.customFetch) {
			return this.customFetch;
		}
		if (typeof window !== 'undefined') {
			return window.fetch.bind(window);
		}
		return fetch;
	}

	private async request<T>(path: string, options?: RequestInit): Promise<T> {
		const url = `${this.baseUrl}${path.startsWith('/') ? path : `/${path}`}`;
		const controller = new AbortController();
		const timeoutId = setTimeout(() => controller.abort(), 6000);

		try {
			const res = await this.fetcher(url, {
				...options,
				signal: controller.signal,
				headers: {
					Accept: 'application/json',
					...options?.headers
				}
			});

			clearTimeout(timeoutId);

			if (!res.ok) {
				const errorText = await res.text().catch(() => 'Unknown Error');
				throw new Error(`API Error [${res.status}]: ${errorText}`);
			}

			return (await res.json()) as T;
		} catch (err: unknown) {
			clearTimeout(timeoutId);
			if (err instanceof Error && err.name === 'AbortError') {
				throw new Error(`API request timed out: ${url}`);
			}
			throw err;
		}
	}

	/**
	 * GET /healthz - Liveness probe
	 */
	async getHealth(): Promise<HealthResponse> {
		return this.request<HealthResponse>('/healthz');
	}

	/**
	 * GET /readyz - Readiness probe
	 */
	async getReady(): Promise<ReadyResponse> {
		return this.request<ReadyResponse>('/readyz');
	}

	/**
	 * GET /api/v1/status - Comprehensive service status
	 */
	async getStatus(): Promise<StatusResponse> {
		return this.request<StatusResponse>('/api/v1/status');
	}

	/**
	 * Ping all diagnostic endpoints and compute roundtrip latency
	 */
	async checkSystemOverview(): Promise<SystemOverview> {
		const start = Date.now();
		try {
			const [health, ready, status] = await Promise.all([
				this.getHealth(),
				this.getReady(),
				this.getStatus()
			]);

			const latencyMs = Date.now() - start;

			return {
				connected: true,
				latencyMs,
				health,
				ready,
				status,
				lastChecked: new Date().toISOString()
			};
		} catch (err: unknown) {
			const latencyMs = Date.now() - start;
			const message = err instanceof Error ? err.message : String(err);
			return {
				connected: false,
				latencyMs,
				lastChecked: new Date().toISOString(),
				errorMessage: message
			};
		}
	}
}

export const apiClient = new ApiClient();
