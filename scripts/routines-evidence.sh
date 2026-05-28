#!/usr/bin/env bash
set -euo pipefail

# Runs the routines E2E flow in the isolated Docker test stack and writes
# human-review artifacts to an ignored local directory.

project="multica-routines-evidence-$$"
compose=(docker compose -f docker-compose.test.yml -p "$project")
run_id="$(date -u +%Y%m%dT%H%M%SZ)"
local_dir="local-artifacts/routines/$run_id"
container_dir="/app/$local_dir"

cleanup() {
  echo ""
  echo "==> Cleaning up project $project..."
  "${compose[@]}" down -v --remove-orphans >/dev/null 2>&1 || true
}
trap cleanup EXIT

mkdir -p "$local_dir"

echo "==> Writing routines evidence to $local_dir"
"${compose[@]}" run --rm \
  -e ROUTINES_EVIDENCE_DIR="$container_dir" \
  e2e sh -c "corepack enable && pnpm exec playwright test e2e/routines.spec.ts"

echo ""
echo "✓ Routines evidence written to $local_dir"
echo "  Summary: $local_dir/summary.md"
