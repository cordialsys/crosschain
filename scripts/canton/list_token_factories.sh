#!/usr/bin/env bash

set -euo pipefail

SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/lib/canton_token_standard.sh"

usage() {
  cat <<'EOF'
List visible Canton token TransferFactory contracts for a party using grpcurl.

Required environment variables:
  PARTICIPANT_HOST   gRPC host:port of the participant
  PARTY_ID           Party whose visible factory contracts should be listed
  AUTH_TOKEN         Bearer token value, unless AUTH_HEADER is provided

Optional environment variables:
  AUTH_HEADER        Full authorization header value
  CONTRACT           Optional token selector in the form <instrument-admin>#<instrument-id>
  PACKAGE_NAME       Defaults to splice-api-token-transfer-instruction-v1
  LEDGER_END         Optional fixed offset. If unset, fetched from GetLedgerEnd.
  RAW                Defaults to 0. Set to 1 to print the raw streaming response.

Output:
  A JSON array of factory-like contracts with:
    - contract_id
    - template
    - admin
    - instrument_id

This is useful for checking which factory the driver can actually see on ledger.
EOF
}

if [[ "${1:-}" == "-h" || "${1:-}" == "--help" ]]; then
  usage
  exit 0
fi

: "${PARTICIPANT_HOST:?PARTICIPANT_HOST is required}"
: "${PARTY_ID:?PARTY_ID is required}"

PACKAGE_NAME="${PACKAGE_NAME:-splice-api-token-transfer-instruction-v1}"
CONTRACT="${CONTRACT:-}"
RAW="${RAW:-0}"
LEDGER_END="${LEDGER_END:-$(canton_get_ledger_end)}"

PACKAGE_ID="$(canton_resolve_package_id_by_name "$PACKAGE_NAME")"
if [[ -z "$PACKAGE_ID" ]]; then
  echo "Could not resolve package id for $PACKAGE_NAME" >&2
  exit 1
fi

TMP="$(mktemp)"
trap 'rm -f "$TMP"' EXIT

jq -n \
  --arg party "$PARTY_ID" \
  --argjson ledgerEnd "$LEDGER_END" \
  '{
    activeAtOffset: $ledgerEnd,
    eventFormat: {
      filtersByParty: {
        ($party): {
          cumulative: []
        }
      },
      verbose: true
    }
  }' \
  | canton_grpc_call "com.daml.ledger.api.v2.StateService/GetActiveContracts" >"$TMP"

if [[ "$RAW" == "1" ]]; then
  cat "$TMP"
  exit 0
fi

jq -s --arg packageId "$PACKAGE_ID" --arg contract "$CONTRACT" '
  def active_contract: (.activeContract // .active_contract // null);
  def created_event: (.createdEvent // .created_event // null);
  def template_id: (.templateId // .template_id // null);
  def create_args: (.createArguments // .create_arguments // null);
  def fields($record):
    if $record == null then []
    else ($record.fields // [])
    end;
  def field($record; $name):
    (fields($record) | map(select(.label == $name)) | first | .value) // null;
  def template_name($id):
    if $id == null then ""
    else (($id.moduleName // $id.module_name // "") + ":" + ($id.entityName // $id.entity_name // ""))
    end;
  def parse_factory($created):
    ($created | create_args) as $args
    | (field($args; "owner")) as $owner
    | (field($args; "amount")) as $amount
    | if $owner != null or $amount != null then
        empty
      else
        (field($args; "admin")) as $admin
        | if $admin == null or ($admin.party // $admin.party_id // "") == "" then
            empty
          else
            (field($args; "symbol")) as $symbol
            | (field($args; "instrumentId")) as $instrument
            | {
                contract_id: ($created.contractId // $created.contract_id // ""),
                template: template_name($created.templateId // $created.template_id),
                admin: ($admin.party // $admin.party_id // ""),
                instrument_id: (
                  if $symbol != null and ($symbol.text // "") != "" then
                    ($symbol.text // "")
                  elif $instrument != null then
                    ((field(($instrument.record // $instrument.record_value // {}); "id").text) // "")
                  else
                    ""
                  end
                )
              }
          end
      end;
  [
    .[]
    | select((active_contract) != null)
    | active_contract
    | created_event as $created
    | select((($created.templateId // $created.template_id // {}).packageId // "") == $packageId)
    | parse_factory($created)
  ]
  | if $contract == "" then .
    else map(select((.admin + "#" + .instrument_id) == $contract))
    end
' "$TMP"
