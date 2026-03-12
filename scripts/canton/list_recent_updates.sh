#!/usr/bin/env bash

set -euo pipefail

SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/lib/canton_token_standard.sh"

usage() {
  cat <<'EOF'
List recent Canton Ledger API updates for a party, focused on token-standard views.

Required environment variables:
  PARTICIPANT_HOST   gRPC host:port of the participant
  PARTY_ID           Party whose recent updates should be listed
  AUTH_TOKEN         Bearer token value, unless AUTH_HEADER is provided

Optional environment variables:
  AUTH_HEADER        Full ledger authorization header
  BEGIN_OFFSET       Optional explicit beginExclusive offset
  END_OFFSET         Optional explicit endInclusive offset
  OFFSET_WINDOW      Defaults to 250 when BEGIN_OFFSET is not set
  HOLDING_PACKAGE    Defaults to splice-api-token-holding-v1
  TRANSFER_PACKAGE   Defaults to splice-api-token-transfer-instruction-v1
  MATCH_TEXT         Optional substring filter applied to the summarized JSON
  RAW                Defaults to 0. Set to 1 to print the raw streaming response

Output:
  A JSON array of summarized transactions with update_id, command_id, record_time,
  creates, and exercises. This is useful for checking whether anything committed
  despite a submit timeout.
EOF
}

if [[ "${1:-}" == "-h" || "${1:-}" == "--help" ]]; then
  usage
  exit 0
fi

: "${PARTICIPANT_HOST:?PARTICIPANT_HOST is required}"
: "${PARTY_ID:?PARTY_ID is required}"

END_OFFSET="${END_OFFSET:-}"
BEGIN_OFFSET="${BEGIN_OFFSET:-}"
OFFSET_WINDOW="${OFFSET_WINDOW:-250}"
HOLDING_PACKAGE="${HOLDING_PACKAGE:-splice-api-token-holding-v1}"
TRANSFER_PACKAGE="${TRANSFER_PACKAGE:-splice-api-token-transfer-instruction-v1}"
MATCH_TEXT="${MATCH_TEXT:-}"
RAW="${RAW:-0}"

if [[ -z "$END_OFFSET" ]]; then
  END_OFFSET="$(canton_get_ledger_end)"
fi

if [[ -z "$BEGIN_OFFSET" ]]; then
  BEGIN_OFFSET="$(( END_OFFSET > OFFSET_WINDOW ? END_OFFSET - OFFSET_WINDOW : 0 ))"
fi

HOLDING_PACKAGE_ID="$(canton_resolve_package_id_by_name "$HOLDING_PACKAGE")"
TRANSFER_PACKAGE_ID="$(canton_resolve_package_id_by_name "$TRANSFER_PACKAGE")"
if [[ -z "$HOLDING_PACKAGE_ID" || -z "$TRANSFER_PACKAGE_ID" ]]; then
  echo "Could not resolve required package ids. Make sure the token-standard DARs are uploaded." >&2
  exit 1
fi

TMP="$(mktemp)"
trap 'rm -f "$TMP"' EXIT

jq -n \
  --arg party "$PARTY_ID" \
  --arg holdingPackageId "$HOLDING_PACKAGE_ID" \
  --arg transferPackageId "$TRANSFER_PACKAGE_ID" \
  --argjson beginOffset "$BEGIN_OFFSET" \
  --argjson endOffset "$END_OFFSET" \
  '{
    beginExclusive: $beginOffset,
    endInclusive: $endOffset,
    updateFormat: {
      includeTransactions: {
        eventFormat: {
          filtersByParty: {
            ($party): {
              cumulative: [
                {
                  interfaceFilter: {
                    interfaceId: {
                      packageId: $holdingPackageId,
                      moduleName: "Splice.Api.Token.HoldingV1",
                      entityName: "Holding"
                    },
                    includeInterfaceView: true,
                    includeCreatedEventBlob: false
                  }
                },
                {
                  interfaceFilter: {
                    interfaceId: {
                      packageId: $transferPackageId,
                      moduleName: "Splice.Api.Token.TransferInstructionV1",
                      entityName: "TransferInstruction"
                    },
                    includeInterfaceView: true,
                    includeCreatedEventBlob: false
                  }
                },
                {
                  interfaceFilter: {
                    interfaceId: {
                      packageId: $transferPackageId,
                      moduleName: "Splice.Api.Token.TransferInstructionV1",
                      entityName: "TransferFactory"
                    },
                    includeInterfaceView: true,
                    includeCreatedEventBlob: false
                  }
                }
              ]
            }
          },
          verbose: true
        },
        transactionShape: "TRANSACTION_SHAPE_LEDGER_EFFECTS"
      }
    }
  }' \
  | canton_grpc_call "com.daml.ledger.api.v2.UpdateService/GetUpdates" >"$TMP"

if [[ "$RAW" == "1" ]]; then
  cat "$TMP"
  exit 0
fi

jq -s --arg matchText "$MATCH_TEXT" '
  def tx: (.transaction // .Transaction // null);
  def ts_to_iso($ts):
    if $ts == null then ""
    elif ($ts | type) == "string" then $ts
    elif ($ts.seconds // null) != null then
      ((($ts.seconds | tonumber) + (($ts.nanos // 0) / 1000000000)) | todateiso8601)
    else ""
    end;
  def ident($id):
    if $id == null then ""
    else (($id.moduleName // $id.module_name // "") + ":" + ($id.entityName // $id.entity_name // ""))
    end;
  def event_list($events):
    if ($events | type) == "array" then $events else [] end;
  [
    .[]
    | tx as $tx
    | select($tx != null)
    | {
        update_id: ($tx.updateId // $tx.update_id // ""),
        command_id: ($tx.commandId // $tx.command_id // ""),
        workflow_id: ($tx.workflowId // $tx.workflow_id // ""),
        offset: ($tx.offset // 0),
        synchronizer_id: ($tx.synchronizerId // $tx.synchronizer_id // ""),
        record_time: ts_to_iso($tx.recordTime // $tx.record_time),
        effective_at: ts_to_iso($tx.effectiveAt // $tx.effective_at),
        creates: [
          event_list($tx.events)[]?
          | (.created // .Created // null) as $created
          | select($created != null)
          | {
              contract_id: ($created.contractId // $created.contract_id // ""),
              template: ident($created.templateId // $created.template_id)
            }
        ],
        exercises: [
          event_list($tx.events)[]?
          | (.exercised // .Exercised // null) as $exercised
          | select($exercised != null)
          | {
              contract_id: ($exercised.contractId // $exercised.contract_id // ""),
              template: ident($exercised.templateId // $exercised.template_id // $exercised.interfaceId // $exercised.interface_id),
              choice: ($exercised.choice // "")
            }
        ]
      }
  ]
  | if $matchText == "" then .
    else map(select((tojson | test($matchText; "i"))))
    end
' "$TMP"
