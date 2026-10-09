/**
 * Types generated from and conforming to api/openapi.yaml
 */

export interface HealthResponse {
	status: string;
	timestamp: string;
	service: string;
}

export interface ReadyResponse {
	status: string;
	timestamp: string;
	checks: Record<string, string>;
}

export interface StatusResponse {
	status: string;
	version: string;
	service: string;
	environment: string;
	uptime_seconds: number;
	timestamp: string;
	components: Record<string, string>;
}

export interface ApiError {
	error: string;
	request_id?: string;
	status?: number;
}

export interface SystemOverview {
	connected: boolean;
	latencyMs: number;
	health?: HealthResponse;
	ready?: ReadyResponse;
	status?: StatusResponse;
	lastChecked: string;
	errorMessage?: string;
}
