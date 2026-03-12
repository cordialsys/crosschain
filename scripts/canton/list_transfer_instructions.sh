#!/usr/bin/env bash

set -euo pipefail

SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/lib/canton_token_standard.sh"

usage() {
  cat <<'EOF'
List Canton token-standard TransferInstruction contracts for a party using grpcurl.

Required environment variables:
  PARTICIPANT_HOST   gRPC host:port of the participant
  PARTY_ID           Party whose transfer instructions should be listed
  AUTH_TOKEN         Bearer token value, unless AUTH_HEADER is provided

Optional environment variables:
  AUTH_HEADER        Full authorization header value
  CONTRACT           Optional token selector in the form <instrument-admin>#<instrument-id>
  PACKAGE_NAME       Defaults to splice-api-token-transfer-instruction-v1
  LEDGER_END         Optional fixed offset. If unset, fetched from GetLedgerEnd.
  RAW                Defaults to 0. Set to 1 to print the raw streaming response.
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

canton_get_transfer_instructions_json "$PARTY_ID" "$PACKAGE_ID" "$LEDGER_END" >"$TMP"

if [[ "$RAW" == "1" ]]; then
  cat "$TMP"
  exit 0
fi

jq -s --arg contract "$CONTRACT" '
  def active_contract: (.activeContract // .active_contract // null);
  def created_event: (.createdEvent // .created_event // null);
  def template_id: (.templateId // .template_id // null);
  def interface_views: (.interfaceViews // .interface_views // []);
  def view_value: (.viewValue // .view_value // null);
  def fields: (.fields // []);
  def field_value($field_list; $name): $field_list[] | select(.label == $name) | .value;
  def status_tag($status):
    if ($status.variant // null) != null then $status.variant.constructor
    elif ($status.enum // null) != null then $status.enum.constructor
    else ""
    end;
  map(select((active_contract) != null))
  | map((active_contract | created_event))
  | map(
      . as $created
      | (interface_views[0] | view_value | fields) as $view_fields
      | (field_value($view_fields; "transfer").record.fields) as $transfer_fields
      | {
          contract_id: ($created.contractId // $created.contract_id),
          template: (
            if template_id == null then ""
            else ((template_id.moduleName // template_id.module_name) + ":" + (template_id.entityName // template_id.entity_name))
            end
          ),
          sender: (field_value($transfer_fields; "sender").party // ""),
          receiver: (field_value($transfer_fields; "receiver").party // ""),
          instrument_admin: (
            (field_value($transfer_fields; "instrumentId").record.fields | field_value(.; "admin").party) // ""
          ),
          instrument_id: (
            (field_value($transfer_fields; "instrumentId").record.fields | field_value(.; "id").text) // ""
          ),
          amount: (field_value($transfer_fields; "amount").numeric // ""),
          status: (status_tag(field_value($view_fields; "status")))
        }
    )
  | if $contract == "" then .
    else map(select((.instrument_admin + "#" + .instrument_id) == $contract))
    end
' "$TMP"
