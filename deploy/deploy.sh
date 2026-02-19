#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_DIR="$(dirname "$SCRIPT_DIR")"
COMPOSE="docker compose -f docker-compose.prod.yml"

cd "$PROJECT_DIR"

# Verify .env.prod exists
if [ ! -f .env.prod ]; then
    echo "ERROR: .env.prod not found. Copy .env.prod.example to .env.prod and fill in values."
    exit 1
fi

# Source .env.prod for variable interpolation
set -a
source .env.prod
set +a

echo "==> Building images..."
$COMPOSE build

echo "==> Running database migrations..."
# Migrations use advisory locks which require a direct connection,
# not the Supavisor transaction-mode pooler (port 6543).
# Override DB_PORT to use Supabase's direct connection (port 5432).
$COMPOSE run --rm -e DB_PORT=5432 api /migrate up

echo "==> Starting services..."
$COMPOSE up -d

echo "==> Cleaning up old images..."
docker image prune -f

echo "==> Done. Services:"
$COMPOSE ps

echo ""
echo "Site should be live at https://${DOMAIN:-localhost}"
