#!/usr/bin/env bash
# Usage: ./scripts/extract-release-notes.sh <version>
#
# Extracts the release notes for a specific version from CHANGELOG.md and
# prints them to stdout.  The version argument should NOT include the leading
# "v" prefix (e.g. pass "0.1.4", not "v0.1.4").
#
# Example:
#   ./scripts/extract-release-notes.sh 0.1.4 > /tmp/release-notes.md
#   goreleaser release --clean --release-notes /tmp/release-notes.md

set -euo pipefail

VERSION="${1:?Usage: $0 <version>}"
CHANGELOG="${CHANGELOG_FILE:-CHANGELOG.md}"

awk -v version="$VERSION" '
  /^## \[/ {
    if (found) exit
    if ($0 ~ "\\[" version "\\]") { found = 1; next }
  }
  found { print }
' "$CHANGELOG"
