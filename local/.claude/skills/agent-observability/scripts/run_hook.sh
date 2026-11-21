#!/bin/sh
set -eu

PROJECT_DIR="${CLAUDE_PROJECT_DIR:-$(cd "$(dirname "$0")"/../../../.. && pwd -P)}"
MAIN_PY="$PROJECT_DIR/.codex/skills/agent-observability/scripts/main.py"

exec python3 "$MAIN_PY" "$@"
