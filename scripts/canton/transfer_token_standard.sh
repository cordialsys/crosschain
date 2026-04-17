#!/usr/bin/env bash

set -euo pipefail

SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/lib/canton_token_standard.sh"

usage() {
  cat <<'EOF'
Transfer a token-standard asset for an internal Canton party using the registry flow.

This uses:
  1. Ledger API to read Holding interface contracts for the sender.
  2. The token-standard transfer registry to fetch factory context.
  3. grpcurl CommandService submission to exercise TransferFactory_Transfer.

Today, the practical transferable implementation on the Canton testnet is Amulet.

Required environment variables:
  PARTICIPANT_HOST   gRPC host:port of the participant
  AUTH_TOKEN         Bearer token value, unless AUTH_HEADER is provided
  SENDER_PARTY       Sender / controller party
  RECEIVER_PARTY     Receiver party
  AMOUNT             Decimal amount, for example 1.2500000000
  CONTRACT           Token selector in the form <instrument-admin>#<instrument-id>

Registry auth:
  SCAN_AUTH_TOKEN    Bearer token for the scan registry/proxy, unless SCAN_AUTH_HEADER is provided

Registry location:
  Either set SCAN_PROXY_URL + SCAN_API_URL
  or set REGISTRY_BASE_URL for direct access.

Optional environment variables:
  AUTH_HEADER        Full ledger authorization header
  SCAN_AUTH_HEADER   Full registry authorization header
  USER_ID            Optional Ledger API user id
  COMMAND_ID         Defaults to ts-transfer-<timestamp>
  SUBMISSION_ID      Defaults to ts-transfer-<timestamp>
  DEDUP_SECONDS      Defaults to 300
  EXECUTE_AFTER_SEC  Defaults to 86400 (24h)
  REASON             Optional reason metadata string
  HOLDING_PACKAGE    Defaults to splice-api-token-holding-v1
  TRANSFER_PACKAGE   Defaults to splice-api-token-transfer-instruction-v1
EOF
}

if [[ "${1:-}" == "-h" || "${1:-}" == "--help" ]]; then
  usage
  exit 0
fi

: "${PARTICIPANT_HOST:?PARTICIPANT_HOST is required}"
: "${SENDER_PARTY:?SENDER_PARTY is required}"
: "${RECEIVER_PARTY:?RECEIVER_PARTY is required}"
: "${AMOUNT:?AMOUNT is required}"
: "${CONTRACT:?CONTRACT is required}"

COMMAND_ID="${COMMAND_ID:-ts-transfer-$(date +%s)}"
SUBMISSION_ID="${SUBMISSION_ID:-ts-transfer-$(date +%s)}"
DEDUP_SECONDS="${DEDUP_SECONDS:-300}"
EXECUTE_AFTER_SEC="${EXECUTE_AFTER_SEC:-86400}"
REASON="${REASON:-}"
HOLDING_PACKAGE="${HOLDING_PACKAGE:-splice-api-token-holding-v1}"
TRANSFER_PACKAGE="${TRANSFER_PACKAGE:-splice-api-token-transfer-instruction-v1}"

if [[ "$CONTRACT" != *"#"* ]]; then
  echo "CONTRACT must be in the form <instrument-admin>#<instrument-id>" >&2
  exit 1
fi
INSTRUMENT_ADMIN="${CONTRACT%%#*}"
INSTRUMENT_ID="${CONTRACT#*#}"

PACKAGE_MAP_JSON="$(canton_build_package_map_json)"
HOLDING_PACKAGE_ID="$(canton_resolve_package_id_by_name "$HOLDING_PACKAGE")"
TRANSFER_PACKAGE_ID="$(canton_resolve_package_id_by_name "$TRANSFER_PACKAGE")"

if [[ -z "$HOLDING_PACKAGE_ID" || -z "$TRANSFER_PACKAGE_ID" ]]; then
  echo "Could not resolve required package ids. Make sure the token-standard interface DARs are uploaded." >&2
  exit 1
fi

LEDGER_END="${LEDGER_END:-$(canton_get_ledger_end)}"
HOLDINGS_JSON="$(canton_get_token_holdings_json "$SENDER_PARTY" "$HOLDING_PACKAGE_ID" "$LEDGER_END")"
MATCHING_HOLDING_CIDS="$(jq -r --arg contract "$CONTRACT" '
  def active_contract: (.activeContract // .active_contract // null);
  def created_event: (.createdEvent // .created_event // null);
  def interface_views: (.interfaceViews // .interface_views // []);
  def view_value: (.viewValue // .view_value // null);
  def fields: (.fields // []);
  [
    .[]
    | select((active_contract) != null)
    | active_contract
    | created_event as $created
    | {
        contract_id: ($created.contractId // $created.contract_id),
        instrument_admin: (
          (interface_views[0] | view_value | fields[]
            | select(.label == "instrumentId")
            | (.value.record // .value.record_value // {}) | fields[]
            | select(.label == "admin")
            | (.value.party // .value.party_id // ""))
        ),
        instrument_id: (
          (interface_views[0] | view_value | fields[]
            | select(.label == "instrumentId")
            | (.value.record // .value.record_value // {}) | fields[]
            | select(.label == "id")
            | (.value.text // ""))
        )
      }
    | select((.instrument_admin + "#" + .instrument_id) == $contract)
    | .contract_id
  ]
' <<<"$HOLDINGS_JSON")"

if [[ "$(jq 'length' <<<"$MATCHING_HOLDING_CIDS")" -eq 0 ]]; then
  echo "No visible holdings found for $SENDER_PARTY and contract $CONTRACT" >&2
  exit 1
fi

REQUESTED_AT_RFC3339="$(date -u +"%Y-%m-%dT%H:%M:%SZ")"
if EXECUTE_BEFORE_RFC3339="$(date -u -v+"${EXECUTE_AFTER_SEC}"S +"%Y-%m-%dT%H:%M:%SZ" 2>/dev/null)"; then
  :
else
  EXECUTE_BEFORE_RFC3339="$(date -u -d "@$(($(date -u +%s) + EXECUTE_AFTER_SEC))" +"%Y-%m-%dT%H:%M:%SZ")"
fi

REGISTRY_REQUEST="$(
  jq -cn \
    --arg sender "$SENDER_PARTY" \
    --arg receiver "$RECEIVER_PARTY" \
    --arg amount "$AMOUNT" \
    --arg instrumentAdmin "$INSTRUMENT_ADMIN" \
    --arg instrumentId "$INSTRUMENT_ID" \
    --arg requestedAt "$REQUESTED_AT_RFC3339" \
    --arg executeBefore "$EXECUTE_BEFORE_RFC3339" \
    --arg reason "$REASON" \
    --argjson inputHoldingCids "$MATCHING_HOLDING_CIDS" \
    '{
      choiceArguments: {
        expectedAdmin: $instrumentAdmin,
        transfer: {
          sender: $sender,
          receiver: $receiver,
          amount: $amount,
          instrumentId: {
            admin: $instrumentAdmin,
            id: $instrumentId
          },
          requestedAt: $requestedAt,
          executeBefore: $executeBefore,
          inputHoldingCids: $inputHoldingCids,
          meta: {
            values: (
              if $reason == "" then {}
              else {"splice.lfdecentralizedtrust.org/reason": $reason}
              end
            )
          }
        },
        extraArgs: {
          context: { values: {} },
          meta: { values: {} }
        }
      }
    }'
)"

REGISTRY_RESPONSE="$(canton_scan_api_post "/registry/transfer-instruction/v1/transfer-factory" "$REGISTRY_REQUEST")"

if ! jq -e '.factoryId and .choiceContext and .choiceContext.disclosedContracts' >/dev/null <<<"$REGISTRY_RESPONSE"; then
  echo "Unexpected transfer registry response:" >&2
  echo "$REGISTRY_RESPONSE" >&2
  exit 1
fi

TRANSFER_KIND="$(jq -r '.transferKind // ""' <<<"$REGISTRY_RESPONSE")"
echo "Registry transfer kind: ${TRANSFER_KIND:-unknown}" >&2

jq -n \
  --arg sender "$SENDER_PARTY" \
  --arg receiver "$RECEIVER_PARTY" \
  --arg amount "$AMOUNT" \
  --arg instrumentAdmin "$INSTRUMENT_ADMIN" \
  --arg instrumentId "$INSTRUMENT_ID" \
  --arg requestedAt "$REQUESTED_AT_RFC3339" \
  --arg executeBefore "$EXECUTE_BEFORE_RFC3339" \
  --arg commandId "$COMMAND_ID" \
  --arg submissionId "$SUBMISSION_ID" \
  --arg dedupDuration "${DEDUP_SECONDS}s" \
  --arg transferPackageId "$TRANSFER_PACKAGE_ID" \
  --arg receiverParty "$RECEIVER_PARTY" \
  --arg userId "${USER_ID:-}" \
  --argjson inputHoldingCids "$MATCHING_HOLDING_CIDS" \
  --argjson packageMap "$PACKAGE_MAP_JSON" \
  --argjson registry "$REGISTRY_RESPONSE" \
  --arg reason "$REASON" \
  '
  def template_ref_to_identifier($pkgMap):
    . as $ref
    | ($ref | sub("^#"; "") | split(":")) as $parts
    | if ($parts | length) != 3 then
        error("invalid template ref: " + $ref)
      else
        {
          packageId: (
            if ($ref | startswith("#")) then
              ($pkgMap[$parts[0]] // error("no package id for package name " + $parts[0]))
            else
              $parts[0]
            end
          ),
          moduleName: $parts[1],
          entityName: $parts[2]
        }
      end;
  def metadata_to_value($meta):
    {
      record: {
        fields: [
          {
            label: "values",
            value: {
              textMap: {
                entries: (($meta.values // {}) | to_entries | map({
                  key: .key,
                  value: { text: .value }
                }))
              }
            }
          }
        ]
      }
    };
  def anyvalue_to_value:
    if .tag == "AV_Text" then
      { variant: { constructor: "AV_Text", value: { text: .value } } }
    elif .tag == "AV_Int" then
      { variant: { constructor: "AV_Int", value: { int64: (.value | tostring) } } }
    elif .tag == "AV_Decimal" then
      { variant: { constructor: "AV_Decimal", value: { numeric: (.value | tostring) } } }
    elif .tag == "AV_Bool" then
      { variant: { constructor: "AV_Bool", value: { bool: .value } } }
    elif .tag == "AV_Time" then
      { variant: { constructor: "AV_Time", value: { timestamp: ((.value | fromdateiso8601 * 1000000 | floor) | tostring) } } }
    elif .tag == "AV_Party" then
      { variant: { constructor: "AV_Party", value: { party: .value } } }
    elif .tag == "AV_ContractId" then
      { variant: { constructor: "AV_ContractId", value: { contractId: .value } } }
    elif .tag == "AV_List" then
      { variant: { constructor: "AV_List", value: { list: { elements: (.value | map(anyvalue_to_value)) } } } }
    elif .tag == "AV_Map" then
      { variant: { constructor: "AV_Map", value: { textMap: { entries: (.value | to_entries | map({
        key: .key,
        value: (.value | anyvalue_to_value)
      })) } } } }
    else
      error("unsupported AnyValue tag " + (.tag // "null"))
    end;
  def choice_context_to_value($ctx):
    {
      record: {
        fields: [
          {
            label: "values",
            value: {
              textMap: {
                entries: (($ctx.values // {}) | to_entries | map({
                  key: .key,
                  value: (.value | anyvalue_to_value)
                }))
              }
            }
          }
        ]
      }
    };
  {
    commands: (
      {
        commandId: $commandId,
        commands: [
          {
            exercise: {
              templateId: {
                packageId: $transferPackageId,
                moduleName: "Splice.Api.Token.TransferInstructionV1",
                entityName: "TransferFactory"
              },
              contractId: $registry.factoryId,
              choice: "TransferFactory_Transfer",
              choiceArgument: {
                record: {
                  fields: [
                    { label: "expectedAdmin", value: { party: $instrumentAdmin } },
                    {
                      label: "transfer",
                      value: {
                        record: {
                          fields: [
                            { label: "sender", value: { party: $sender } },
                            { label: "receiver", value: { party: $receiver } },
                            { label: "amount", value: { numeric: $amount } },
                            {
                              label: "instrumentId",
                              value: {
                                record: {
                                  fields: [
                                    { label: "admin", value: { party: $instrumentAdmin } },
                                    { label: "id", value: { text: $instrumentId } }
                                  ]
                                }
                              }
                            },
                            { label: "requestedAt", value: { timestamp: (($requestedAt | fromdateiso8601 * 1000000 | floor) | tostring) } },
                            { label: "executeBefore", value: { timestamp: (($executeBefore | fromdateiso8601 * 1000000 | floor) | tostring) } },
                            { label: "inputHoldingCids", value: { list: { elements: ($inputHoldingCids | map({ contractId: . })) } } },
                            {
                              label: "meta",
                              value: metadata_to_value({
                                values: (
                                  if $reason == "" then {}
                                  else {"splice.lfdecentralizedtrust.org/reason": $reason}
                                  end
                                )
                              })
                            }
                          ]
                        }
                      }
                    },
                    {
                      label: "extraArgs",
                      value: {
                        record: {
                          fields: [
                            { label: "context", value: choice_context_to_value($registry.choiceContext.choiceContextData) },
                            { label: "meta", value: metadata_to_value({values:{}}) }
                          ]
                        }
                      }
                    }
                  ]
                }
              }
            }
          }
        ],
        disclosedContracts: ($registry.choiceContext.disclosedContracts | map({
          templateId: (.templateId | template_ref_to_identifier($packageMap)),
          contractId: .contractId,
          createdEventBlob: .createdEventBlob,
          synchronizerId: .synchronizerId
        })),
        deduplicationDuration: $dedupDuration,
        actAs: [$sender],
        readAs: [$sender],
        submissionId: $submissionId,
        synchronizerId: ($registry.choiceContext.disclosedContracts[0].synchronizerId)
      }
      | if $userId != "" then . + { userId: $userId } else . end
    ),
    transactionFormat: {
      eventFormat: {
        filtersByParty: {
          ($sender): {
            cumulative: [
              {
                interfaceFilter: {
                  interfaceId: {
                    packageId: $transferPackageId,
                    moduleName: "Splice.Api.Token.TransferInstructionV1",
                    entityName: "TransferInstruction"
                  },
                  includeInterfaceView: true,
                  includeCreatedEventBlob: true
                }
              }
            ]
          },
          ($receiverParty): {
            cumulative: [
              {
                interfaceFilter: {
                  interfaceId: {
                    packageId: $transferPackageId,
                    moduleName: "Splice.Api.Token.TransferInstructionV1",
                    entityName: "TransferInstruction"
                  },
                  includeInterfaceView: true,
                  includeCreatedEventBlob: true
                }
              }
            ]
          }
        },
        verbose: true
      },
      transactionShape: "TRANSACTION_SHAPE_ACS_DELTA"
    }
  }' \
  | canton_grpc_call "com.daml.ledger.api.v2.CommandService/SubmitAndWaitForTransaction"
