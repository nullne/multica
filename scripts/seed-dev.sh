#!/usr/bin/env sh
set -eu

# seed-dev.sh — bootstrap a dev environment with user, workspace, agents, and CLI config.
# Runs inside a minimal Alpine container with curl and jq.
# Idempotent: safe to run multiple times.

API_URL="${API_URL:-http://backend:8080}"
SEED_EMAIL="${SEED_EMAIL:-dev@multica.local}"
CONFIG_DIR="${CONFIG_DIR:-/root/.multica}"
CONFIG_FILE="${CONFIG_DIR}/config.json"

apk add --no-cache curl jq > /dev/null 2>&1 || true

api() {
  method="$1"; path="$2"; shift 2
  curl -s -X "$method" "${API_URL}${path}" \
    -H "Content-Type: application/json" \
    -H "Authorization: Bearer ${TOKEN:-}" \
    -H "X-Workspace-ID: ${WORKSPACE_ID:-}" \
    "$@"
}

create_agent() {
  name="$1"; description="$2"; instructions="$3"
  AGENTS=$(api GET /api/agents)
  EXISTS=$(echo "$AGENTS" | jq -r --arg n "$name" '[.[] | select(.name == $n)] | length')
  if [ "$EXISTS" -gt 0 ]; then
    echo "  Agent already exists: ${name} (skipped)"
    return
  fi
  BODY=$(jq -n \
    --arg name "$name" \
    --arg desc "$description" \
    --arg inst "$instructions" \
    '{name:$name, description:$desc, instructions:$inst, providers:["claude"], visibility:"workspace", max_concurrent_tasks:1}')
  RESP=$(api POST /api/agents -d "$BODY")
  ANAME=$(echo "$RESP" | jq -r '.name // empty')
  if [ -z "$ANAME" ]; then
    echo "  ERROR: failed to create agent '${name}'"
    echo "  $RESP"
    exit 1
  fi
  echo "  Agent created: ${ANAME}"
}

echo "==> Seeding dev environment..."

# 1. Authenticate (master code 888888 works in non-production).
#    send-code may return 500 if no email service, but still inserts the DB row.
echo "  Authenticating as ${SEED_EMAIL}..."
api POST /auth/send-code -d "{\"email\":\"${SEED_EMAIL}\"}" > /dev/null 2>&1
LOGIN=$(api POST /auth/verify-code -d "{\"email\":\"${SEED_EMAIL}\",\"code\":\"888888\"}")
TOKEN=$(echo "$LOGIN" | jq -r '.token // empty')
if [ -z "$TOKEN" ]; then
  echo "  ERROR: authentication failed"
  echo "  $LOGIN"
  exit 1
fi
echo "  Authenticated."

# 2. Get workspace
WORKSPACES=$(api GET /api/workspaces)
WORKSPACE_ID=$(echo "$WORKSPACES" | jq -r '.[0].id // empty')
WORKSPACE_NAME=$(echo "$WORKSPACES" | jq -r '.[0].name // empty')
if [ -z "$WORKSPACE_ID" ]; then
  echo "  ERROR: no workspace found"
  exit 1
fi
echo "  Workspace: ${WORKSPACE_NAME} (${WORKSPACE_ID})"

# 3. Register daemon (no runtimes — daemon will auto-install providers on startup)
echo "  Registering daemon..."
REG_BODY="{\"workspace_id\":\"${WORKSPACE_ID}\",\"daemon_id\":\"dev-daemon\",\"device_name\":\"dev-container\",\"cli_version\":\"dev\",\"runtimes\":[]}"
api POST /api/daemon/register -d "$REG_BODY" > /dev/null
echo "  Daemon registered."

# 4. Create two agents: Coder (executor) and Reviewer (verifier)
echo "  Creating agents..."

create_agent \
  "Coder" \
  "Development agent — writes code, fixes bugs, implements features" \
  "You are a software engineer. When assigned an issue, read the issue description carefully, understand the requirements, and implement the solution. Write clean, tested code. When you are done, set the issue status to in_review."

create_agent \
  "Reviewer" \
  "Code review agent — verifies implementations against acceptance criteria" \
  "You are a code reviewer. When asked to verify work, check the implementation against the acceptance criteria. Run tests if applicable. If everything passes, approve. If there are issues, provide specific feedback on what needs to be fixed."

# 5. Write CLI config for daemon
mkdir -p "$CONFIG_DIR"
cat > "$CONFIG_FILE" <<EOF
{
  "server_url": "${API_URL}",
  "token": "${TOKEN}",
  "workspace_id": "${WORKSPACE_ID}",
  "watched_workspaces": [
    {"id": "${WORKSPACE_ID}", "name": "${WORKSPACE_NAME}"}
  ]
}
EOF
echo "  CLI config written to ${CONFIG_FILE}"

echo "==> Dev seed complete."
echo "  Coder  — assign issues to this agent"
echo "  Reviewer — set as verifier for verification loop"
