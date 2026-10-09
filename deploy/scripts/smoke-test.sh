#!/usr/bin/env bash
set -euo pipefail

# ==============================================================================
# Triage Deployment Smoke Test & Health Verification Suite
# Verifies ingress, Go backend, SvelteKit frontend, metrics, and billing catalog.
# ==============================================================================

TARGET_HOST="${1:-http://localhost}"
MAX_RETRIES="${2:-12}"
SLEEP_INTERVAL="${3:-5}"

echo "========================================================"
echo " Running Smoke Tests against: ${TARGET_HOST}"
echo " Max Retries: ${MAX_RETRIES} | Interval: ${SLEEP_INTERVAL}s"
echo "========================================================"

check_endpoint() {
    local name="$1"
    local endpoint="$2"
    local expected_status="$3"
    local url="${TARGET_HOST}${endpoint}"
    local count=1

    echo -n "==> Testing ${name} (${endpoint})... "

    while [ "${count}" -le "${MAX_RETRIES}" ]; do
        response_code=$(curl -s -o /dev/null -w "%{http_code}" --connect-timeout 5 --max-time 10 "${url}" || echo "000")

        if [ "${response_code}" = "${expected_status}" ]; then
            echo "PASSED (HTTP ${response_code})"
            return 0
        fi

        echo -n "[retry ${count}/${MAX_RETRIES}: got ${response_code}] "
        sleep "${SLEEP_INTERVAL}"
        count=$((count + 1))
    done

    echo "FAILED! Expected HTTP ${expected_status}, got ${response_code} from ${url}"
    return 1
}

echo ""
echo "1. Ingress & Connectivity Probes:"
check_endpoint "Caddy Ingress Probe" "/caddy-healthz" "200" || true # optional if caddy not bound

echo ""
echo "2. Backend Core Diagnostics:"
check_endpoint "Backend Liveness Probe" "/healthz" "200"
check_endpoint "Backend Readiness Probe" "/readyz" "200"
check_endpoint "Telemetry Prometheus Metrics" "/metrics" "200"
check_endpoint "System Status API" "/api/v1/status" "200"

echo ""
echo "3. Stripe Billing & Plans Catalog:"
check_endpoint "Public Billing Plans Catalog" "/api/v1/billing/plans" "200"

echo ""
echo "4. Frontend Web Console:"
check_endpoint "SvelteKit Incident Response Console" "/" "200"

echo ""
echo "========================================================"
echo " All Deployment Smoke Tests Passed Successfully!"
echo " Target is Healthy and Ready for Traffic."
echo "========================================================"
exit 0
