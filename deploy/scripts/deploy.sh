#!/usr/bin/env bash
set -euo pipefail

# ==============================================================================
# Triage Zero-Downtime Deployment & Automated Rollback Script
# ==============================================================================

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "${SCRIPT_DIR}/../.." && pwd)"
DEPLOY_DIR="${ROOT_DIR}/deploy"

COMPOSE_FILE="${DEPLOY_DIR}/docker-compose.prod.yml"
if [ ! -f "${COMPOSE_FILE}" ]; then
    COMPOSE_FILE="${DEPLOY_DIR}/docker-compose.yml"
fi

IMAGE_TAG="${1:-latest}"
PREV_TAG="${2:-}"
TARGET_URL="${3:-http://localhost}"

echo "=========================================================="
echo " Starting Continuous Deployment Pipeline"
echo " Target Tag:    ${IMAGE_TAG}"
echo " Compose File:  ${COMPOSE_FILE}"
echo " Smoke Target:  ${TARGET_URL}"
echo "=========================================================="

export IMAGE_TAG

# 1. Pull New Container Images
echo "==> 1. Pulling latest container images..."
docker compose -f "${COMPOSE_FILE}" pull api worker web mcp caddy || {
    echo "Warning: Image pull failed or using local development images."
}

# 2. Run Database Migrations Before Routing Traffic
echo "==> 2. Applying database migrations (Goose schema up)..."
docker compose -f "${COMPOSE_FILE}" run --rm api /app/seed || {
    echo "ERROR: Database migration failed! Aborting deployment."
    exit 1
}

# 3. Rolling Update of Services
echo "==> 3. Performing rolling container update..."
docker compose -f "${COMPOSE_FILE}" up -d --no-deps --remove-orphans api worker mcp web caddy

# 4. Post-Deployment Smoke Verification Gate
echo "==> 4. Running post-deployment health & smoke test suite..."
if "${SCRIPT_DIR}/smoke-test.sh" "${TARGET_URL}" 12 5; then
    echo "=========================================================="
    echo " Deployment Succeeded! Version ${IMAGE_TAG} is live."
    echo "=========================================================="

    # Prune dangling images
    docker image prune -f --filter "dangling=true" || true
    exit 0
else
    echo "=========================================================="
    echo " CRITICAL: Smoke tests failed! Initiating rollback..."
    echo "=========================================================="

    # Capture diagnostic logs
    echo "==> Dumping recent service logs for investigation:"
    docker compose -f "${COMPOSE_FILE}" logs --tail=50 api web caddy

    if [ -n "${PREV_TAG}" ]; then
        echo "==> Rolling back to previous stable tag: ${PREV_TAG}..."
        IMAGE_TAG="${PREV_TAG}" docker compose -f "${COMPOSE_FILE}" up -d --no-deps api worker mcp web caddy
        "${SCRIPT_DIR}/smoke-test.sh" "${TARGET_URL}" 6 5 || true
    fi

    exit 1
fi
