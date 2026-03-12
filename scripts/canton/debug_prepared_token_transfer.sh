#!/usr/bin/env bash

set -euo pipefail

SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/lib/canton_token_standard.sh"

usage() {
  cat <<'EOF'
Prepare a Canton token transfer and print the exact pre-submit metadata.

This mirrors the crosschain Canton token transfer ledger-fallback shape:
  - actAs: [sender]
  - readAs: [sender, instrument-admin] when admin differs from sender
  - disclosedContracts: the selected token factory created_event_blob

Required environment variables:
  PARTICIPANT_HOST   gRPC host:port of the participant
  AUTH_TOKEN         Bearer token value, unless AUTH_HEADER is provided
  SENDER_PARTY       Sender party
  RECEIVER_PARTY     Receiver party
  CONTRACT           Token selector in the form <instrument-admin>#<instrument-id>
  AMOUNT             Decimal amount, for example 1.000000

Optional environment variables:
  AUTH_HEADER        Full ledger authorization header
  USER_ID            Optional Ledger API user id
  COMMAND_ID         Defaults to debug-token-transfer-<timestamp>
  SUBMISSION_ID      Defaults to debug-token-transfer-<timestamp>
  DEDUP_SECONDS      Defaults to 300
  EXECUTE_AFTER_SEC  Defaults to 86400 (24h)
  SYNCHRONIZER_ID    Optional synchronizer/domain id
  HOLDING_PACKAGE    Defaults to splice-api-token-holding-v1
  TRANSFER_PACKAGE   Defaults to splice-api-token-transfer-instruction-v1
  RAW                Defaults to 0. Set to 1 to print the raw PrepareSubmission response

Output:
  A JSON object containing:
    - selected factory contract/template/admin/instrument
    - prepare request actAs/readAs/disclosed contract ids
    - prepared transaction hash and hashing scheme
    - optional prepared transaction JSON
EOF
}

if [[ "${1:-}" == "-h" || "${1:-}" == "--help" ]]; then
  usage
  exit 0
fi

: "${PARTICIPANT_HOST:?PARTICIPANT_HOST is required}"
: "${SENDER_PARTY:?SENDER_PARTY is required}"
: "${RECEIVER_PARTY:?RECEIVER_PARTY is required}"
: "${CONTRACT:?CONTRACT is required}"
: "${AMOUNT:?AMOUNT is required}"

COMMAND_ID="${COMMAND_ID:-debug-token-transfer-$(date +%s)}"
SUBMISSION_ID="${SUBMISSION_ID:-debug-token-transfer-$(date +%s)}"
DEDUP_SECONDS="${DEDUP_SECONDS:-300}"
EXECUTE_AFTER_SEC="${EXECUTE_AFTER_SEC:-86400}"
HOLDING_PACKAGE="${HOLDING_PACKAGE:-splice-api-token-holding-v1}"
TRANSFER_PACKAGE="${TRANSFER_PACKAGE:-splice-api-token-transfer-instruction-v1}"
RAW="${RAW:-0}"

if [[ "$CONTRACT" != *"#"* ]]; then
  echo "CONTRACT must be in the form <instrument-admin>#<instrument-id>" >&2
  exit 1
fi
INSTRUMENT_ADMIN="${CONTRACT%%#*}"
INSTRUMENT_ID="${CONTRACT#*#}"

TRANSFER_PACKAGE_ID="$(canton_resolve_package_id_by_name "$TRANSFER_PACKAGE")"
if [[ -z "$TRANSFER_PACKAGE_ID" ]]; then
  echo "Could not resolve package id for $TRANSFER_PACKAGE" >&2
  exit 1
fi

MATCHING_HOLDING_CIDS="$(
  PARTICIPANT_HOST="$PARTICIPANT_HOST" \
  AUTH_TOKEN="${AUTH_TOKEN:-}" \
  AUTH_HEADER="${AUTH_HEADER:-}" \
  PARTY_ID="$SENDER_PARTY" \
  CONTRACT="$CONTRACT" \
  PACKAGE_NAME="$HOLDING_PACKAGE" \
  bash "$SCRIPT_DIR/list_token_holdings.sh" \
    | jq -c '[.[] | .contract_id]'
)"
if [[ "$(jq -r 'length' <<<"$MATCHING_HOLDING_CIDS")" -eq 0 ]]; then
  echo "No visible holdings found for $SENDER_PARTY and ${CONTRACT}" >&2
  exit 1
fi

FACTORY_SUMMARY_JSON="$(
  PARTICIPANT_HOST="$PARTICIPANT_HOST" \
  AUTH_TOKEN="${AUTH_TOKEN:-}" \
  AUTH_HEADER="${AUTH_HEADER:-}" \
  PARTY_ID="$INSTRUMENT_ADMIN" \
  CONTRACT="$CONTRACT" \
  PACKAGE_NAME="$TRANSFER_PACKAGE" \
  bash "$SCRIPT_DIR/list_token_factories.sh"
)"
FACTORY_COUNT="$(jq -r 'length' <<<"$FACTORY_SUMMARY_JSON")"
if [[ "$FACTORY_COUNT" -eq 0 ]]; then
  echo "No visible token factory found for ${CONTRACT} on admin view ${INSTRUMENT_ADMIN}" >&2
  exit 1
fi
if [[ "$FACTORY_COUNT" -gt 1 ]]; then
  echo "Multiple visible token factories found for ${CONTRACT}; refine the investigation first." >&2
  jq '.' <<<"$FACTORY_SUMMARY_JSON" >&2
  exit 1
fi
FACTORY_CONTRACT_ID="$(jq -r '.[0].contract_id' <<<"$FACTORY_SUMMARY_JSON")"

LEDGER_END="${LEDGER_END:-$(canton_get_ledger_end)}"
ADMIN_CONTRACTS_RAW="$(
  jq -n \
    --arg party "$INSTRUMENT_ADMIN" \
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
    | canton_grpc_call "com.daml.ledger.api.v2.StateService/GetActiveContracts"
)"

FACTORY_DISCLOSED_JSON="$(
  jq -s --arg contractId "$FACTORY_CONTRACT_ID" '
    def active_contract: (.activeContract // .active_contract // null);
    def created_event: (.createdEvent // .created_event // null);
    [
      .[]
      | select((active_contract) != null)
      | active_contract
      | created_event
      | select((.contractId // .contract_id // "") == $contractId)
      | {
          templateId: (.templateId // .template_id),
          contractId: (.contractId // .contract_id),
          createdEventBlob: (.createdEventBlob // .created_event_blob)
        }
    ]
    | first // empty
  ' <<<"$ADMIN_CONTRACTS_RAW"
)"
if [[ -z "$FACTORY_DISCLOSED_JSON" ]]; then
  echo "Failed to resolve createdEventBlob for factory ${FACTORY_CONTRACT_ID}" >&2
  exit 1
fi

REQUESTED_AT_RFC3339="$(date -u +"%Y-%m-%dT%H:%M:%SZ")"
if EXECUTE_BEFORE_RFC3339="$(date -u -v+"${EXECUTE_AFTER_SEC}"S +"%Y-%m-%dT%H:%M:%SZ" 2>/dev/null)"; then
  :
else
  EXECUTE_BEFORE_RFC3339="$(date -u -d "@$(($(date -u +%s) + EXECUTE_AFTER_SEC))" +"%Y-%m-%dT%H:%M:%SZ")"
fi

READ_AS_JSON="$(jq -cn --arg sender "$SENDER_PARTY" --arg admin "$INSTRUMENT_ADMIN" '
  if $sender == $admin then [$sender] else [$sender, $admin] end
')"

PREPARE_REQUEST="$(
  jq -n \
    --arg transferPackageId "$TRANSFER_PACKAGE_ID" \
    --arg factoryCid "$FACTORY_CONTRACT_ID" \
    --arg sender "$SENDER_PARTY" \
    --arg receiver "$RECEIVER_PARTY" \
    --arg admin "$INSTRUMENT_ADMIN" \
    --arg instrumentId "$INSTRUMENT_ID" \
    --arg amount "$AMOUNT" \
    --arg requestedAt "$REQUESTED_AT_RFC3339" \
    --arg executeBefore "$EXECUTE_BEFORE_RFC3339" \
    --arg userId "${USER_ID:-}" \
    --arg commandId "$COMMAND_ID" \
    --arg submissionId "$SUBMISSION_ID" \
    --arg dedupDuration "${DEDUP_SECONDS}s" \
    --arg synchronizerId "${SYNCHRONIZER_ID:-}" \
    --argjson inputHoldingCids "$MATCHING_HOLDING_CIDS" \
    --argjson readAs "$READ_AS_JSON" \
    --argjson disclosedContract "$FACTORY_DISCLOSED_JSON" \
    '{
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
                contractId: $factoryCid,
                choice: "TransferFactory_Transfer",
                choiceArgument: {
                  record: {
                    fields: [
                      { label: "expectedAdmin", value: { party: $admin } },
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
                                      { label: "admin", value: { party: $admin } },
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
                                value: {
                                  record: {
                                    fields: [
                                      { label: "values", value: { textMap: { entries: [] } } }
                                    ]
                                  }
                                }
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
                              {
                                label: "context",
                                value: {
                                  record: {
                                    fields: [
                                      { label: "values", value: { textMap: { entries: [] } } }
                                    ]
                                  }
                                }
                              },
                              {
                                label: "meta",
                                value: {
                                  record: {
                                    fields: [
                                      { label: "values", value: { textMap: { entries: [] } } }
                                    ]
                                  }
                                }
                              }
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
          deduplicationDuration: $dedupDuration,
          actAs: [$sender],
          readAs: $readAs,
          disclosedContracts: [$disclosedContract],
          submissionId: $submissionId
        }
        | if $userId != "" then . + { userId: $userId } else . end
        | if $synchronizerId != "" then . + { synchronizerId: $synchronizerId } else . end
      )
    }'
)"

PREPARE_RESPONSE="$(printf '%s' "$PREPARE_REQUEST" | canton_grpc_call "com.daml.ledger.api.v2.interactive.InteractiveSubmissionService/PrepareSubmission")"

if [[ "$RAW" == "1" ]]; then
  printf '%s\n' "$PREPARE_RESPONSE"
  exit 0
fi

jq -n \
  --arg contract "$CONTRACT" \
  --arg sender "$SENDER_PARTY" \
  --arg receiver "$RECEIVER_PARTY" \
  --arg amount "$AMOUNT" \
  --argjson holdings "$MATCHING_HOLDING_CIDS" \
  --argjson factory "$(jq '.[0]' <<<"$FACTORY_SUMMARY_JSON")" \
  --argjson prepareReq "$PREPARE_REQUEST" \
  --argjson prepareResp "$PREPARE_RESPONSE" \
  '{
    contract: $contract,
    sender: $sender,
    receiver: $receiver,
    amount: $amount,
    selected_factory: $factory,
    input_holding_cids: $holdings,
    prepare_request: {
      act_as: ($prepareReq.commands.actAs // $prepareReq.commands[0].actAs // []),
      read_as: ($prepareReq.commands.readAs // $prepareReq.commands[0].readAs // []),
      disclosed_contract_ids: (
        ($prepareReq.commands.disclosedContracts // $prepareReq.commands[0].disclosedContracts // [])
        | map(.contractId)
      ),
      submission_id: ($prepareReq.commands.submissionId // $prepareReq.commands[0].submissionId // ""),
      synchronizer_id: ($prepareReq.commands.synchronizerId // $prepareReq.commands[0].synchronizerId // "")
    },
    prepare_response: {
      prepared_transaction_hash_b64: ($prepareResp.prepared_transaction_hash // $prepareResp.preparedTransactionHash // ""),
      hashing_scheme_version: ($prepareResp.hashing_scheme_version // $prepareResp.hashingSchemeVersion // ""),
      prepared_transaction: ($prepareResp.prepared_transaction // $prepareResp.preparedTransaction // null),
      cost_estimation: ($prepareResp.cost_estimation // $prepareResp.costEstimation // null)
    }
  }'
