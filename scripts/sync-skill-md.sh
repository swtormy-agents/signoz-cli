#!/usr/bin/env bash
# Copy canonical agent skill into skills/signoz/ (must match internal/cli/SKILL.md).
set -euo pipefail
cd "$(dirname "$0")/.."
cp internal/cli/SKILL.md skills/signoz/SKILL.md
