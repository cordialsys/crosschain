#!/usr/bin/env bash

set -euo pipefail

usage() {
  cat <<'EOF'
Build a Canton transfer object in the validator-style ExternalPartySubmission shape.

This is the most likely schema expected by Explorer "Proof of Transfer" UIs:
{
  "party_id": "...",
  "transaction": "<base64 PreparedTransaction proto>",
  "signed_tx_hash": "<hex ed25519 signature>",
  "public_key": "<hex ed25519 public key>"
}

Required environment variables:
  PARTY_ID            Submitting/signing party id
  TRANSACTION_B64     Base64-encoded PreparedTransaction proto
  SIGNED_TX_HASH_HEX  Hex-encoded ed25519 signature over the prepared tx hash
  PUBLIC_KEY_HEX      Hex-encoded ed25519 public key

Optional environment variables:
  COMPACT             Defaults to 0. Set to 1 for one-line JSON
  VALIDATE            Defaults to 1. Set to 0 to skip basic shape checks

Examples:
  export PARTY_ID="party::1220..."
  export TRANSACTION_B64="Cg..."
  export SIGNED_TX_HASH_HEX="deadbeef..."
  export PUBLIC_KEY_HEX="abcd..."
  bash scripts/canton/build_transfer_object.sh
EOF
}

if [[ "${1:-}" == "-h" || "${1:-}" == "--help" ]]; then
  usage
  exit 0
fi

: "${PARTY_ID:?PARTY_ID is required}"
: "${TRANSACTION_B64:?TRANSACTION_B64 is required}"
: "${SIGNED_TX_HASH_HEX:?SIGNED_TX_HASH_HEX is required}"
: "${PUBLIC_KEY_HEX:?PUBLIC_KEY_HEX is required}"

COMPACT="${COMPACT:-0}"
VALIDATE="${VALIDATE:-1}"

require_cmd() {
  local cmd="$1"
  if ! command -v "$cmd" >/dev/null 2>&1; then
    echo "Missing required command: $cmd" >&2
    exit 1
  fi
}

require_cmd jq
require_cmd base64

if [[ "$VALIDATE" == "1" ]]; then
  if ! printf '%s' "$TRANSACTION_B64" | base64 -d >/dev/null 2>&1; then
    echo "TRANSACTION_B64 is not valid base64" >&2
    exit 1
  fi

  if ! [[ "$SIGNED_TX_HASH_HEX" =~ ^[0-9a-fA-F]+$ ]]; then
    echo "SIGNED_TX_HASH_HEX must be hex" >&2
    exit 1
  fi
  if ! [[ "$PUBLIC_KEY_HEX" =~ ^[0-9a-fA-F]+$ ]]; then
    echo "PUBLIC_KEY_HEX must be hex" >&2
    exit 1
  fi
fi

if [[ "$COMPACT" == "1" ]]; then
  jq -cn \
    --arg partyId "$PARTY_ID" \
    --arg tx "$TRANSACTION_B64" \
    --arg sig "$SIGNED_TX_HASH_HEX" \
    --arg pub "$PUBLIC_KEY_HEX" \
    '{party_id: $partyId, transaction: $tx, signed_tx_hash: $sig, public_key: $pub}'
else
  jq -n \
    --arg partyId "$PARTY_ID" \
    --arg tx "$TRANSACTION_B64" \
    --arg sig "$SIGNED_TX_HASH_HEX" \
    --arg pub "$PUBLIC_KEY_HEX" \
    '{party_id: $partyId, transaction: $tx, signed_tx_hash: $sig, public_key: $pub}'
fi
