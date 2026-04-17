#!/usr/bin/env bash

set -euo pipefail

SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/lib/canton_token_standard.sh"

usage() {
  cat <<'EOF'
List recent Scan events/verdicts for Canton debugging.

Required environment variables:
  SCAN_AUTH_TOKEN    Bearer token value, unless SCAN_AUTH_HEADER is provided

Location:
  Either set SCAN_PROXY_URL + SCAN_API_URL
  or set SCAN_API_URL directly

Optional environment variables:
  SCAN_AUTH_HEADER   Full scan authorization header
  PARTY_ID           Filter events by verdict.submitting_parties
  MATCH_TEXT         Optional substring filter applied to the event JSON
  PAGE_SIZE          Defaults to 50
  AFTER_RECORD_TIME  Defaults to 30 minutes ago (UTC)
  AFTER_MIGRATION_ID Defaults to 0
  RAW                Defaults to 0. Set to 1 to print the raw response

Output:
  A JSON array summarizing scan events and verdicts, useful for identifying
  the update_id and confirming parties involved in a timed-out transaction.
EOF
}

if [[ "${1:-}" == "-h" || "${1:-}" == "--help" ]]; then
  usage
  exit 0
fi

PAGE_SIZE="${PAGE_SIZE:-50}"
AFTER_MIGRATION_ID="${AFTER_MIGRATION_ID:-0}"
PARTY_ID="${PARTY_ID:-}"
MATCH_TEXT="${MATCH_TEXT:-}"
RAW="${RAW:-0}"

if [[ -z "${AFTER_RECORD_TIME:-}" ]]; then
  if AFTER_RECORD_TIME="$(date -u -v-30M +"%Y-%m-%dT%H:%M:%SZ" 2>/dev/null)"; then
    :
  else
    AFTER_RECORD_TIME="$(date -u -d "@$(($(date -u +%s) - 1800))" +"%Y-%m-%dT%H:%M:%SZ")"
  fi
fi

REQUEST="$(
  jq -cn \
    --argjson pageSize "$PAGE_SIZE" \
    --argjson afterMigrationId "$AFTER_MIGRATION_ID" \
    --arg afterRecordTime "$AFTER_RECORD_TIME" \
    '{
      page_size: $pageSize,
      after: {
        after_migration_id: $afterMigrationId,
        after_record_time: $afterRecordTime
      }
    }'
)"

RESPONSE="$(canton_scan_api_post "/api/scan/v0/events" "$REQUEST")"

if [[ "$RAW" == "1" ]]; then
  printf '%s\n' "$RESPONSE"
  exit 0
fi

jq --arg party "$PARTY_ID" --arg matchText "$MATCH_TEXT" '
  def events_list:
    .events // .Events // .transactions // .Transactions // [];
  def summary($item):
    {
      update_id: (
        $item.verdict.update_id
        // $item.verdict.updateId
        // $item.update.update_id
        // $item.update.updateId
        // ""
      ),
      record_time: (
        $item.verdict.record_time
        // $item.verdict.recordTime
        // $item.update.record_time
        // $item.update.recordTime
        // ""
      ),
      finalization_time: ($item.verdict.finalization_time // $item.verdict.finalizationTime // ""),
      verdict_result: ($item.verdict.verdict_result // $item.verdict.verdictResult // ""),
      mediator_group: ($item.verdict.mediator_group // $item.verdict.mediatorGroup // ""),
      submitting_parties: ($item.verdict.submitting_parties // $item.verdict.submittingParties // []),
      has_update: ($item.update != null),
      view_count: (($item.verdict.transaction_views // $item.verdict.transactionViews // []) | length)
    };
  events_list
  | map(select(
      ($party == "" or ((.verdict.submitting_parties // .verdict.submittingParties // []) | index($party) != null))
      and
      ($matchText == "" or (tojson | test($matchText; "i")))
    ))
  | map(summary(.))
' <<<"$RESPONSE"
