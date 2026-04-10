#!/usr/bin/env bash
set -euo pipefail

# Run test services inside an isolated Docker Compose project.
# Each invocation gets a unique project name so parallel runs never conflict.
#
# Usage: bash scripts/docker-test.sh <service> [service ...]
# Example: bash scripts/docker-test.sh typecheck ts-test go-test e2e

if [ $# -eq 0 ]; then
  echo "Usage: $0 <service> [service ...]"
  echo "Services: typecheck, ts-test, go-test, e2e"
  exit 1
fi

project="multica-test-$$"
compose="docker compose -f docker-compose.test.yml -p $project"

cleanup() {
  echo ""
  echo "==> Cleaning up project $project..."
  $compose down -v --remove-orphans 2>/dev/null || true
}
trap cleanup EXIT

for service in "$@"; do
  echo "==> Running $service..."
  $compose run --rm "$service"
done
