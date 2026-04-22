#!/usr/bin/env bash

set -euo pipefail

usage() {
  cat <<'EOF'
Fetch a Canton transfer object for a private transaction by update id.

This script supports two sources:

1. Participant JSON API transaction-tree endpoint (preferred when available)
   - Returns the legacy JsTransactionTree object, which is the closest match to
     what Explorer-like UIs usually expect for private transfer proof.
2. Ledger API gRPC GetUpdateById (fallback)
   - Returns the participant-visible transaction/update payload as JSON.

Required environment variables:
  UPDATE_ID           Canton update id to fetch
  PARTY_ID            Party whose private view should be used

For JSON API mode:
  LEDGER_JSON_API_URL Base URL of the participant JSON API, e.g. https://.../ledger-api
  AUTH_TOKEN          Bearer token value, unless AUTH_HEADER is provided

For gRPC mode:
  PARTICIPANT_HOST    gRPC host:port of the participant
  AUTH_TOKEN          Bearer token value, unless AUTH_HEADER is provided

Optional environment variables:
  AUTH_HEADER         Full authorization header value, e.g. "Bearer ey..." or "Basic abc..."
  SOURCE              "auto" (default), "json-api", or "grpc"
  RAW                 Defaults to 0. Set to 1 to print the full upstream response.
  COMPACT             Defaults to 0. Set to 1 to print compact one-line JSON for pasting.
  INCLUDE_BLOBS       Defaults to 1 for gRPC mode. Include created event blobs.
  VERBOSE             Defaults to 1 for gRPC mode. Include verbose value labels.

Examples:
  export UPDATE_ID="1220..."
  export PARTY_ID="party::1220..."
  export PARTICIPANT_HOST="ledgerapi-participant.example.com:443"
  export AUTH_TOKEN="..."
  bash scripts/canton/get_transfer_object.sh

  export LEDGER_JSON_API_URL="https://participant.example.com"
  export SOURCE="json-api"
  export COMPACT=1
  bash scripts/canton/get_transfer_object.sh
EOF
}

if [[ "${1:-}" == "-h" || "${1:-}" == "--help" ]]; then
  usage
  exit 0
fi

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=/Users/pawel/Source/crosschain/scripts/canton/lib/canton_token_standard.sh
source "$SCRIPT_DIR/lib/canton_token_standard.sh"

: "${UPDATE_ID:?UPDATE_ID is required}"
: "${PARTY_ID:?PARTY_ID is required}"

SOURCE="${SOURCE:-auto}"
RAW="${RAW:-0}"
COMPACT="${COMPACT:-0}"
INCLUDE_BLOBS="${INCLUDE_BLOBS:-1}"
VERBOSE="${VERBOSE:-1}"

json_api_call() {
  local path="$1"
  local auth_header
  local tmp

  : "${LEDGER_JSON_API_URL:?LEDGER_JSON_API_URL is required for json-api mode}"
  auth_header="$(canton_resolve_auth_header)"
  tmp="$(mktemp)"
  if ! curl -fsSL \
    -H "Authorization: $auth_header" \
    "${LEDGER_JSON_API_URL%/}${path}" >"$tmp"; then
    cat "$tmp" >&2 || true
    rm -f "$tmp"
    return 1
  fi
  cat "$tmp"
  rm -f "$tmp"
}

fetch_via_json_api() {
  local encoded_party
  encoded_party="$(jq -rn --arg value "$PARTY_ID" '$value|@uri')"
  json_api_call "/v2/updates/transaction-tree-by-id/${UPDATE_ID}?parties=${encoded_party}"
}

fetch_via_grpc() {
  local include_blobs_json
  local verbose_json

  : "${PARTICIPANT_HOST:?PARTICIPANT_HOST is required for grpc mode}"

  if [[ "$INCLUDE_BLOBS" == "1" ]]; then
    include_blobs_json=true
  else
    include_blobs_json=false
  fi
  if [[ "$VERBOSE" == "1" ]]; then
    verbose_json=true
  else
    verbose_json=false
  fi

  jq -n \
    --arg updateId "$UPDATE_ID" \
    --arg party "$PARTY_ID" \
    --argjson includeBlobs "$include_blobs_json" \
    --argjson verbose "$verbose_json" \
    '{
      updateId: $updateId,
      updateFormat: {
        includeTransactions: {
          eventFormat: {
            filtersByParty: {
              ($party): {
                cumulative: [
                  {
                    wildcardFilter: {
                      includeCreatedEventBlob: $includeBlobs
                    }
                  }
                ]
              }
            },
            verbose: $verbose
          },
          transactionShape: "TRANSACTION_SHAPE_LEDGER_EFFECTS"
        }
      }
    }' \
    | canton_grpc_call "com.daml.ledger.api.v2.UpdateService/GetUpdateById"
}

response=""
mode_used=""

case "$SOURCE" in
  json-api)
    response="$(fetch_via_json_api)"
    mode_used="json-api"
    ;;
  grpc)
    response="$(fetch_via_grpc)"
    mode_used="grpc"
    ;;
  auto)
    if [[ -n "${LEDGER_JSON_API_URL:-}" ]]; then
      if response="$(fetch_via_json_api)"; then
        mode_used="json-api"
      else
        echo "JSON API fetch failed, falling back to gRPC GetUpdateById." >&2
        response="$(fetch_via_grpc)"
        mode_used="grpc"
      fi
    else
      response="$(fetch_via_grpc)"
      mode_used="grpc"
    fi
    ;;
  *)
    echo "Unsupported SOURCE: $SOURCE" >&2
    exit 1
    ;;
esac

if [[ "$RAW" == "1" ]]; then
  if [[ "$COMPACT" == "1" ]]; then
    jq -c '.' <<<"$response"
  else
    jq '.' <<<"$response"
  fi
  exit 0
fi

if [[ "$mode_used" == "json-api" ]]; then
  if [[ "$COMPACT" == "1" ]]; then
    jq -c '.transaction' <<<"$response"
  else
    jq '.transaction' <<<"$response"
  fi
  exit 0
fi

if [[ "$COMPACT" == "1" ]]; then
  jq -c '(.transaction // .update.transaction // .updateTransaction // .update.transactionTree // .transactionTree)' <<<"$response"
else
  jq '(.transaction // .update.transaction // .updateTransaction // .update.transactionTree // .transactionTree)' <<<"$response"
fi
