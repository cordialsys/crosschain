#!/usr/bin/env bash

set -euo pipefail

SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"

usage() {
  cat <<'EOF'
Run the main Canton token-timeout debugging checks in one place.

Required environment variables:
  PARTICIPANT_HOST   gRPC host:port of the participant
  PARTY_ID           Party to inspect (usually the submitter/external party)
  AUTH_TOKEN         Bearer token value, unless AUTH_HEADER is provided

Optional environment variables:
  CONTRACT           Optional <instrument-admin>#<instrument-id> selector
  UPDATE_ID          Optional update_id to inspect via Scan
  OFFSET_WINDOW      Defaults to 250

Scan-related optional environment variables:
  SCAN_AUTH_TOKEN    Bearer token value, unless SCAN_AUTH_HEADER is provided
  SCAN_AUTH_HEADER   Full scan authorization header
  SCAN_PROXY_URL     Optional scan proxy URL
  SCAN_API_URL       Scan base URL
EOF
}

if [[ "${1:-}" == "-h" || "${1:-}" == "--help" ]]; then
  usage
  exit 0
fi

: "${PARTICIPANT_HOST:?PARTICIPANT_HOST is required}"
: "${PARTY_ID:?PARTY_ID is required}"

CONTRACT="${CONTRACT:-}"
OFFSET_WINDOW="${OFFSET_WINDOW:-250}"

echo "== Holdings" >&2
PARTY_ID="$PARTY_ID" CONTRACT="$CONTRACT" \
  bash "$SCRIPT_DIR/list_token_holdings.sh"

echo >&2
echo "== Transfer Instructions" >&2
PARTY_ID="$PARTY_ID" CONTRACT="$CONTRACT" \
  bash "$SCRIPT_DIR/list_transfer_instructions.sh"

echo >&2
echo "== Recent Participant Updates" >&2
PARTY_ID="$PARTY_ID" MATCH_TEXT="${CONTRACT:-}" OFFSET_WINDOW="$OFFSET_WINDOW" \
  bash "$SCRIPT_DIR/list_recent_updates.sh"

if [[ -n "${SCAN_AUTH_TOKEN:-}${SCAN_AUTH_HEADER:-}" ]]; then
  echo >&2
  echo "== Recent Scan Events" >&2
  PARTY_ID="$PARTY_ID" MATCH_TEXT="${CONTRACT:-}" \
    bash "$SCRIPT_DIR/list_scan_events.sh"

  if [[ -n "${UPDATE_ID:-}" ]]; then
    echo >&2
    echo "== Scan Event Detail" >&2
    UPDATE_ID="$UPDATE_ID" bash "$SCRIPT_DIR/get_scan_event.sh"
  fi
fi
