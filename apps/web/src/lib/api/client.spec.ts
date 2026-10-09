import { describe, it, expect, vi } from 'vitest';
import { ApiClient } from './client';

describe('ApiClient', () => {
	it('fetches healthz successfully', async () => {
		const mockResponse = {
			status: 'ok',
			timestamp: '2026-10-09T07:00:00Z',
			service: 'triage-api'
		};

		const mockFetch = vi.fn().mockResolvedValue({
			ok: true,
			json: async () => mockResponse
		} as Response);

		const client = new ApiClient('http://test-host:8080', mockFetch);
		const result = await client.getHealth();

		expect(mockFetch).toHaveBeenCalledWith(
			'http://test-host:8080/healthz',
			expect.objectContaining({
				headers: expect.objectContaining({ Accept: 'application/json' })
			})
		);
		expect(result).toEqual(mockResponse);
	});

	it('fetches status successfully', async () => {
		const mockResponse = {
			status: 'online',
			version: '0.1.0',
			service: 'triage-api',
			environment: 'development',
			uptime_seconds: 42,
			timestamp: '2026-10-09T07:00:00Z',
			components: { database: 'configured', redis: 'configured' }
		};

		const mockFetch = vi.fn().mockResolvedValue({
			ok: true,
			json: async () => mockResponse
		} as Response);

		const client = new ApiClient('http://test-host:8080', mockFetch);
		const result = await client.getStatus();

		expect(result.service).toBe('triage-api');
		expect(result.status).toBe('online');
		expect(result.version).toBe('0.1.0');
	});

	it('computes system overview and handles errors gracefully', async () => {
		const mockFetch = vi.fn().mockRejectedValue(new Error('Connection refused'));

		const client = new ApiClient('http://offline-host:8080', mockFetch);
		const overview = await client.checkSystemOverview();

		expect(overview.connected).toBe(false);
		expect(overview.errorMessage).toContain('Connection refused');
		expect(typeof overview.latencyMs).toBe('number');
	});
});
