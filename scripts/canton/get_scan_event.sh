#!/usr/bin/env bash

set -euo pipefail

SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/lib/canton_token_standard.sh"

usage() {
  cat <<'EOF'
Fetch a specific Scan event/verdict by update_id.

Required environment variables:
  SCAN_AUTH_TOKEN    Bearer token value, unless SCAN_AUTH_HEADER is provided
  UPDATE_ID          The update_id to inspect

Location:
  Either set SCAN_PROXY_URL + SCAN_API_URL
  or set SCAN_API_URL directly

Optional environment variables:
  SCAN_AUTH_HEADER   Full scan authorization header
  INCLUDE_UPDATE     Defaults to 1. Also fetch /v2/updates/{update_id}
  PAGE_SIZE          Defaults to 200 for the POST /v0/events fallback
  AFTER_RECORD_TIME  Defaults to 2 hours ago (UTC) for the POST /v0/events fallback
  AFTER_MIGRATION_ID Defaults to 0 for the POST /v0/events fallback
  RAW                Defaults to 0. Set to 1 to print the raw event payload

Output:
  A normalized JSON object including the verdict result, mediator group,
  and confirming_parties for each transaction view.
EOF
}

if [[ "${1:-}" == "-h" || "${1:-}" == "--help" ]]; then
  usage
  exit 0
fi

: "${UPDATE_ID:?UPDATE_ID is required}"

INCLUDE_UPDATE="${INCLUDE_UPDATE:-1}"
PAGE_SIZE="${PAGE_SIZE:-200}"
AFTER_MIGRATION_ID="${AFTER_MIGRATION_ID:-0}"
RAW="${RAW:-0}"

if [[ -z "${AFTER_RECORD_TIME:-}" ]]; then
  if AFTER_RECORD_TIME="$(date -u -v-2H +"%Y-%m-%dT%H:%M:%SZ" 2>/dev/null)"; then
    :
  else
    AFTER_RECORD_TIME="$(date -u -d "@$(($(date -u +%s) - 7200))" +"%Y-%m-%dT%H:%M:%SZ")"
  fi
fi

EVENT_RESPONSE=""
if ! EVENT_RESPONSE="$(canton_scan_api_get "/api/scan/v0/events/${UPDATE_ID}" 2>/dev/null)"; then
  FALLBACK_REQUEST="$(
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
  FALLBACK_RESPONSE="$(canton_scan_api_post "/api/scan/v0/events" "$FALLBACK_REQUEST")"
  EVENT_RESPONSE="$(
    jq -c --arg updateId "$UPDATE_ID" '
      (
        .events // .Events // .transactions // .Transactions // []
      )
      | map(select(
          (
            .verdict.update_id
            // .verdict.updateId
            // .update.update_id
            // .update.updateId
            // ""
          ) == $updateId
        ))
      | first // empty
    ' <<<"$FALLBACK_RESPONSE"
  )"
  if [[ -z "$EVENT_RESPONSE" ]]; then
    echo "Could not resolve Scan event for update_id $UPDATE_ID via GET or POST /api/scan/v0/events fallback." >&2
    exit 1
  fi
fi

if [[ "$RAW" == "1" ]]; then
  printf '%s\n' "$EVENT_RESPONSE"
  exit 0
fi

UPDATE_RESPONSE='null'
if [[ "$INCLUDE_UPDATE" == "1" ]]; then
  if UPDATE_RESPONSE="$(canton_scan_api_get "/api/scan/v2/updates/${UPDATE_ID}" 2>/dev/null)"; then
    :
  else
    UPDATE_RESPONSE='null'
  fi
fi

jq -n --argjson event "$EVENT_RESPONSE" --argjson update "$UPDATE_RESPONSE" '
  def event_obj:
    $event.event // $event.Event // $event;
  def normalize_view_item($item; $fallbackId):
    if ($item | type) == "object" then
      ($item + {view_id: ($item.view_id // $item.viewId // $fallbackId)})
    else
      {
        view_id: $fallbackId,
        raw: $item
      }
    end;
  def normalize_views($raw):
    if ($raw | type) == "array" then
      ($raw | to_entries | map(normalize_view_item(.value; (.value.view_id // .value.viewId // (.key | tostring)))))
    elif ($raw | type) == "object" then
      ($raw | to_entries | map(normalize_view_item(.value; .key)))
    else
      []
    end;
  def views($verdict):
    normalize_views($verdict.transaction_views // $verdict.transactionViews // [])
    | map({
        view_id: (.view_id // .viewId // ""),
        informees: (.informees // []),
        confirming_parties: (.confirming_parties // .confirmingParties // null),
        sub_views: (.sub_views // .subViews // [])
      });
  {
    update_id: (
      event_obj.verdict.update_id
      // event_obj.verdict.updateId
      // event_obj.update.update_id
      // event_obj.update.updateId
      // ""
    ),
    record_time: (
      event_obj.verdict.record_time
      // event_obj.verdict.recordTime
      // event_obj.update.record_time
      // event_obj.update.recordTime
      // ""
    ),
    finalization_time: (event_obj.verdict.finalization_time // event_obj.verdict.finalizationTime // ""),
    verdict_result: (event_obj.verdict.verdict_result // event_obj.verdict.verdictResult // ""),
    mediator_group: (event_obj.verdict.mediator_group // event_obj.verdict.mediatorGroup // ""),
    submitting_parties: (event_obj.verdict.submitting_parties // event_obj.verdict.submittingParties // []),
    transaction_views: views(event_obj.verdict // {}),
    update: $update
  }
'
