#!/usr/bin/env bash
# ───────────────────────────────────────────────────────────────────
# Seed the boilerplate database with an initial admin user.
#
# Usage:
#   bash scripts/seed.sh
#
# Requires psql and a running PostgreSQL container.
# ───────────────────────────────────────────────────────────────────
set -euo pipefail

DB_CONTAINER="${DB_CONTAINER:-boilerplate-postgres-1}"
DB_USER="${DB_USER:-app_user}"
DB_NAME="${DB_NAME:-boilerplate}"

# Detect psql — try docker exec first, then local psql
if docker ps --format '{{.Names}}' 2>/dev/null | grep -q "^${DB_CONTAINER}$"; then
  PSQL="docker exec -i ${DB_CONTAINER} psql -U ${DB_USER} -d ${DB_NAME}"
elif command -v psql &>/dev/null; then
  PSQL="psql -U ${DB_USER} -d ${DB_NAME}"
else
  echo "[ERR] Cannot find psql or running Docker container '${DB_CONTAINER}'"
  echo "      Start the stack first:  docker compose up -d"
  exit 1
fi

# Seed admin user (bcrypt cost 12 hash for "password123")
$PSQL <<'SQL'
INSERT INTO users (id, name, email, password_hash)
VALUES (
  '00000000-0000-0000-0000-000000000001',
  'Admin',
  'admin@boilerplate.com',
  '$2b$12$Tu8vShrx2rnZIGzKQ3nR5O30TNCV7P75DZwrPjHEJDBALMdZVO9/K'
)
ON CONFLICT (email) DO NOTHING;
SQL

echo "✓ Seed complete — admin@boilerplate.com / password123"
