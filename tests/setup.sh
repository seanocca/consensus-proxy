#!/usr/bin/env bash
#
# Setup script for the beacon node test environment.
# Generates the required JWT secret shared between execution and consensus clients.
#
# Usage:
#   ./tests/setup.sh

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
JWT_FILE="${SCRIPT_DIR}/jwt.hex"

if [[ -f "$JWT_FILE" ]]; then
  echo "JWT secret already exists at ${JWT_FILE}, skipping generation."
else
  openssl rand -hex 32 > "$JWT_FILE"
  echo "JWT secret generated at ${JWT_FILE}"
fi
