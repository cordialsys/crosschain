#!/usr/bin/env bash

set -euo pipefail

SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/lib/canton_token_standard.sh"

cat <<'USAGE' >/dev/null
Required environment:
  SCAN_AUTH_TOKEN or SCAN_AUTH_HEADER
  SCAN_API_URL or REGISTRY_BASE_URL
Optional:
  SCAN_PROXY_URL   Validator HTTP proxy URL
  PAGE_SIZE        Number of instruments per page (default: 100)
  PAGE_TOKEN       Starting page token
  ALL=1            Follow nextPageToken until exhausted
USAGE

PAGE_SIZE="${PAGE_SIZE:-100}"
PAGE_TOKEN="${PAGE_TOKEN:-}"
ALL="${ALL:-0}"

registry_info_json="$(canton_scan_api_get "/registry/metadata/v1/info")"

admin_id="$(
  jq -r '.adminId // empty' <<<"$registry_info_json"
)"
if [[ -z "$admin_id" ]]; then
  echo "Unexpected registry info response:" >&2
  echo "$registry_info_json" >&2
  exit 1
fi

fetch_page() {
  local page_token="$1"
  local path="/registry/metadata/v1/instruments?pageSize=${PAGE_SIZE}"
  if [[ -n "$page_token" ]]; then
    path="${path}&pageToken=${page_token}"
  fi
  canton_scan_api_get "$path"
}

pages=()
next_token="$PAGE_TOKEN"
while :; do
  page_json="$(fetch_page "$next_token")"
  pages+=("$page_json")
  if [[ "$ALL" != "1" ]]; then
    break
  fi
  next_token="$(jq -r '.nextPageToken // empty' <<<"$page_json")"
  if [[ -z "$next_token" ]]; then
    break
  fi
done

{
  printf '%s\n' "${pages[@]}"
} | jq -s --arg adminId "$admin_id" '
  {
    admin_id: $adminId,
    instruments:
      (map(.instruments // [])
       | add
       | map({
           id,
           name,
           symbol,
           decimals,
           total_supply: .totalSupply,
           total_supply_as_of: .totalSupplyAsOf,
           supported_apis: .supportedApis
         })),
    next_page_token:
      (if length == 0 then ""
       else (.[-1].nextPageToken // "")
       end)
  }
'
