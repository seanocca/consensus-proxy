#!/usr/bin/env bash
#
# Setup script for the beacon node test environment.
# Generates the required JWT secret and downloads checkpoint state files
# needed by clients that don't support direct checkpoint sync URLs.
#
# Usage:
#   ./tests/setup.sh

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
JWT_FILE="${SCRIPT_DIR}/jwt.hex"
ENV_FILE="${SCRIPT_DIR}/.env"
CHECKPOINT_URL="${CHECKPOINT_SYNC_URL:-https://beaconstate-sepolia.chainsafe.io}"

# ── JWT secret ───────────────────────────────────────────────────────

if [[ -f "$JWT_FILE" ]]; then
  echo "JWT secret already exists at ${JWT_FILE}, skipping generation."
else
  openssl rand -hex 32 > "$JWT_FILE"
  echo "JWT secret generated at ${JWT_FILE}"
fi

# ── Write .env ────────────────────────────────────────────────────────
# Docker Compose reads .env from the compose file directory.
: > "$ENV_FILE"

# ── Genesis states ─────────────────────────────────────────────────────
# Prysm does not bundle genesis states for testnets. We download them
# so Prysm can start without requiring an external checkpoint sync URL.

GENESIS_DIR="${SCRIPT_DIR}/genesis"
mkdir -p "$GENESIS_DIR"

SEPOLIA_GENESIS="${GENESIS_DIR}/sepolia-genesis.ssz"
if [[ -f "$SEPOLIA_GENESIS" ]]; then
  echo "Sepolia genesis state already exists, skipping download."
else
  echo "Downloading Sepolia genesis state for Prysm..."
  if curl -sfL -o "$SEPOLIA_GENESIS" \
    "https://media.githubusercontent.com/media/eth-clients/sepolia/main/metadata/genesis.ssz"; then
    echo "Sepolia genesis state saved to ${SEPOLIA_GENESIS}"
  else
    echo "Warning: failed to download Sepolia genesis state."
    echo "Prysm may not start without --genesis-state or --genesis-beacon-api-url."
    rm -f "$SEPOLIA_GENESIS"
  fi
fi

# ── Finalized checkpoint state ─────────────────────────────────────────
# Nimbus's checkpoint sync providers don't serve light client bootstrap
# data, so --external-beacon-api-url alone won't work. We download the
# latest finalized state as an SSZ file (~3 MB) and pass it via
# --finalized-checkpoint-state. Re-downloaded every run to stay current.

CHECKPOINT_DIR="${SCRIPT_DIR}/checkpoint"
mkdir -p "$CHECKPOINT_DIR"
FINALIZED_STATE="${CHECKPOINT_DIR}/finalized-state.ssz"

echo "Downloading finalized checkpoint state from ${CHECKPOINT_URL}..."
if curl -sf -H "Accept: application/octet-stream" \
  -o "$FINALIZED_STATE" \
  "${CHECKPOINT_URL}/eth/v2/debug/beacon/states/finalized"; then
  echo "Finalized checkpoint state saved to ${FINALIZED_STATE}"
else
  echo "Warning: failed to download finalized checkpoint state."
  echo "Nimbus will sync from genesis (slow)."
  rm -f "$FINALIZED_STATE"
fi
