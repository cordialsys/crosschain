#!/usr/bin/env bash

set -euo pipefail

usage() {
  cat <<'EOF'
List Canton token-standard holdings for a party using grpcurl.

Required environment variables:
  PARTICIPANT_HOST   gRPC host:port of the participant
  PARTY_ID           Party whose holdings should be listed
  AUTH_TOKEN         Bearer token value, unless AUTH_HEADER is provided

Optional environment variables:
  AUTH_HEADER        Full authorization header value, e.g. "Bearer ey..." or "Basic abc..."
  CONTRACT           Optional token selector in the form <instrument-admin>#<instrument-id>
  PACKAGE_NAME       Defaults to splice-api-token-holding-v1
  LEDGER_END         Optional fixed offset. If unset, fetched from GetLedgerEnd.
  RAW                Defaults to 0. Set to 1 to print the raw streaming response.

Output:
  A JSON array of holdings with:
    - contract_id
    - owner
    - instrument_admin
    - instrument_id
    - amount
    - template
EOF
}

if [[ "${1:-}" == "-h" || "${1:-}" == "--help" ]]; then
  usage
  exit 0
fi

: "${PARTICIPANT_HOST:?PARTICIPANT_HOST is required}"
: "${PARTY_ID:?PARTY_ID is required}"
: "${AUTH_TOKEN:?AUTH_TOKEN is required}"

AUTH_HEADER="${AUTH_HEADER:-Bearer $AUTH_TOKEN}"
CONTRACT="${CONTRACT:-}"
PACKAGE_NAME="${PACKAGE_NAME:-splice-api-token-holding-v1}"
LEDGER_END="${LEDGER_END:-}"
RAW="${RAW:-0}"

grpc_call() {
  local method="$1"
  local tmp
  tmp="$(mktemp)"
  if ! grpcurl \
    -H "authorization: $AUTH_HEADER" \
    -d @ \
    "$PARTICIPANT_HOST" \
    "$method" >"$tmp"; then
    cat "$tmp" >&2 || true
    rm -f "$tmp"
    return 1
  fi
  cat "$tmp"
  rm -f "$tmp"
}

resolve_package_id() {
  local response
  response="$(printf '{}' | grpc_call "com.daml.ledger.api.v2.admin.PackageManagementService/ListKnownPackages")" || return 1
  if ! jq -e '(.package_details // .packageDetails) != null' >/dev/null <<<"$response"; then
    echo "Unexpected ListKnownPackages response:" >&2
    echo "$response" >&2
    return 1
  fi
  jq -r --arg packageName "$PACKAGE_NAME" '
    (.package_details // .packageDetails)
    | map(select(.name == $packageName))
    | sort_by((.known_since // .knownSince))
    | last
    | (.package_id // .packageId // empty)
  ' <<<"$response"
}

if [[ -z "$LEDGER_END" ]]; then
  ledger_end_response="$(printf '{}' | grpc_call "com.daml.ledger.api.v2.StateService/GetLedgerEnd")" || exit 1
  if ! jq -e '(.offset // .ledger_end // .ledgerEnd) != null' >/dev/null <<<"$ledger_end_response"; then
    echo "Unexpected GetLedgerEnd response:" >&2
    echo "$ledger_end_response" >&2
    exit 1
  fi
  LEDGER_END="$(jq -r '(.offset // .ledger_end // .ledgerEnd)' <<<"$ledger_end_response")"
fi

PACKAGE_ID="$(resolve_package_id)"
if [[ -z "$PACKAGE_ID" ]]; then
  echo "Could not resolve package id for $PACKAGE_NAME" >&2
  exit 1
fi

tmp="$(mktemp)"
trap 'rm -f "$tmp"' EXIT

jq -n \
  --arg party "$PARTY_ID" \
  --arg packageId "$PACKAGE_ID" \
  --argjson ledgerEnd "$LEDGER_END" \
  '{
    activeAtOffset: $ledgerEnd,
    eventFormat: {
      filtersByParty: {
        ($party): {
          cumulative: [
            {
              interfaceFilter: {
                interfaceId: {
                  packageId: $packageId,
                  moduleName: "Splice.Api.Token.HoldingV1",
                  entityName: "Holding"
                },
                includeInterfaceView: true,
                includeCreatedEventBlob: false
              }
            }
          ]
        }
      },
      verbose: true
    }
  }' \
  | grpc_call "com.daml.ledger.api.v2.StateService/GetActiveContracts" >"$tmp"

if ! jq -e 'type == "object" or type == "array"' >/dev/null <"$tmp"; then
  echo "Unexpected GetActiveContracts response:" >&2
  cat "$tmp" >&2
  exit 1
fi

if jq -e 'type == "object" and ((has("activeContract") or has("active_contract")) | not) and ((has("offsetCheckpoint") or has("offset_checkpoint")) | not) and has("contractEntry")' >/dev/null <"$tmp"; then
  jq '.activeContract = (.contractEntry.activeContract // .contractEntry.active_contract // null)' "$tmp" >"${tmp}.normalized"
  mv "${tmp}.normalized" "$tmp"
fi

if [[ "$RAW" == "1" ]]; then
  cat "$tmp"
  exit 0
fi

jq -s --arg contract "$CONTRACT" '
  def active_contract: (.activeContract // .active_contract // null);
  def created_event: (.createdEvent // .created_event // null);
  def template_id: (.templateId // .template_id // null);
  def interface_views: (.interfaceViews // .interface_views // []);
  def view_value: (.viewValue // .view_value // null);
  def fields: (.fields // []);
  def value: (.value // {});
  def record: (.record // {});
  map(select((active_contract) != null))
  | map((active_contract | created_event))
  | map({
      contract_id: (.contractId // .contract_id),
      template: (
        if template_id == null then ""
        else ((template_id.moduleName // template_id.module_name) + ":" + (template_id.entityName // template_id.entity_name))
        end
      ),
      owner: (
        interface_views[0] | view_value | fields[]
        | select(.label == "owner")
        | (.value.party // .value.party_id // "")
      ),
      instrument_admin: (
        interface_views[0] | view_value | fields[]
        | select(.label == "instrumentId")
        | (.value.record // .value.record_value // {}) | fields[]
        | select(.label == "admin")
        | (.value.party // .value.party_id // "")
      ),
      instrument_id: (
        interface_views[0] | view_value | fields[]
        | select(.label == "instrumentId")
        | (.value.record // .value.record_value // {}) | fields[]
        | select(.label == "id")
        | (.value.text // "")
      ),
      amount: (
        interface_views[0] | view_value | fields[]
        | select(.label == "amount")
        | (.value.numeric // "")
      )
    })
  | if $contract == "" then .
    else map(select((.instrument_admin + "#" + .instrument_id) == $contract))
    end
' "$tmp"
