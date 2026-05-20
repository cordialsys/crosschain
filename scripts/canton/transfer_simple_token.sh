#!/usr/bin/env bash

set -euo pipefail

SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/lib/canton_token_standard.sh"

usage() {
  cat <<'EOF'
Transfer the simple example token by exercising its TransferFactory directly.

Required environment variables:
  PARTICIPANT_HOST      gRPC host:port of the participant
  AUTH_TOKEN            Bearer token value, unless AUTH_HEADER is provided
  FACTORY_CONTRACT_ID   Contract id of the SimpleTokenFactory
  SENDER_PARTY          Sender / controller party
  RECEIVER_PARTY        Receiver party
  ADMIN_PARTY           Token admin / issuer party
  SYMBOL                Instrument id text
  AMOUNT                Decimal amount, for example 1.0

Optional environment variables:
  AUTH_HEADER           Full authorization header value
  USER_ID               Optional Ledger API user id
  COMMAND_ID            Defaults to simple-token-transfer-<timestamp>
  SUBMISSION_ID         Defaults to transfer-simple-token-<timestamp>
  DEDUP_SECONDS         Defaults to 300
  EXECUTE_AFTER_SEC     Defaults to 86400 (24h)
  SYNCHRONIZER_ID       Optional synchronizer/domain id
  HOLDING_PACKAGE       Defaults to splice-api-token-holding-v1
  TRANSFER_PACKAGE      Defaults to splice-api-token-transfer-instruction-v1
EOF
}

if [[ "${1:-}" == "-h" || "${1:-}" == "--help" ]]; then
  usage
  exit 0
fi

: "${PARTICIPANT_HOST:?PARTICIPANT_HOST is required}"
: "${FACTORY_CONTRACT_ID:?FACTORY_CONTRACT_ID is required}"
: "${SENDER_PARTY:?SENDER_PARTY is required}"
: "${RECEIVER_PARTY:?RECEIVER_PARTY is required}"
: "${ADMIN_PARTY:?ADMIN_PARTY is required}"
: "${SYMBOL:?SYMBOL is required}"
: "${AMOUNT:?AMOUNT is required}"

COMMAND_ID="${COMMAND_ID:-simple-token-transfer-$(date +%s)}"
SUBMISSION_ID="${SUBMISSION_ID:-transfer-simple-token-$(date +%s)}"
DEDUP_SECONDS="${DEDUP_SECONDS:-300}"
EXECUTE_AFTER_SEC="${EXECUTE_AFTER_SEC:-86400}"
HOLDING_PACKAGE="${HOLDING_PACKAGE:-splice-api-token-holding-v1}"
TRANSFER_PACKAGE="${TRANSFER_PACKAGE:-splice-api-token-transfer-instruction-v1}"

TRANSFER_PACKAGE_ID="$(canton_resolve_package_id_by_name "$TRANSFER_PACKAGE")"

if [[ -z "$TRANSFER_PACKAGE_ID" ]]; then
  echo "Could not resolve package id for $TRANSFER_PACKAGE" >&2
  exit 1
fi

CONTRACT_KEY="${ADMIN_PARTY}#${SYMBOL}"
MATCHING_HOLDING_CIDS="$(
  PARTICIPANT_HOST="$PARTICIPANT_HOST" \
  AUTH_TOKEN="${AUTH_TOKEN:-}" \
  AUTH_HEADER="${AUTH_HEADER:-}" \
  PARTY_ID="$SENDER_PARTY" \
  CONTRACT="$CONTRACT_KEY" \
  PACKAGE_NAME="$HOLDING_PACKAGE" \
  bash "$SCRIPT_DIR/list_token_holdings.sh" \
    | jq -c '[.[] | .contract_id]'
)"

if [[ "$(jq -r 'length' <<<"$MATCHING_HOLDING_CIDS")" -eq 0 ]]; then
  echo "No visible holdings found for $SENDER_PARTY and ${CONTRACT_KEY}" >&2
  exit 1
fi

REQUESTED_AT_RFC3339="$(date -u +"%Y-%m-%dT%H:%M:%SZ")"
if EXECUTE_BEFORE_RFC3339="$(date -u -v+"${EXECUTE_AFTER_SEC}"S +"%Y-%m-%dT%H:%M:%SZ" 2>/dev/null)"; then
  :
else
  EXECUTE_BEFORE_RFC3339="$(date -u -d "@$(($(date -u +%s) + EXECUTE_AFTER_SEC))" +"%Y-%m-%dT%H:%M:%SZ")"
fi

build_payload() {
  if [[ -n "${SYNCHRONIZER_ID:-}" ]]; then
    jq -n \
      --arg transferPackageId "$TRANSFER_PACKAGE_ID" \
      --arg factoryCid "$FACTORY_CONTRACT_ID" \
      --arg sender "$SENDER_PARTY" \
      --arg receiver "$RECEIVER_PARTY" \
      --arg admin "$ADMIN_PARTY" \
      --arg symbol "$SYMBOL" \
      --arg amount "$AMOUNT" \
      --arg requestedAt "$REQUESTED_AT_RFC3339" \
      --arg executeBefore "$EXECUTE_BEFORE_RFC3339" \
      --arg userId "${USER_ID:-}" \
      --arg commandId "$COMMAND_ID" \
      --arg submissionId "$SUBMISSION_ID" \
      --arg dedupDuration "${DEDUP_SECONDS}s" \
      --arg synchronizerId "$SYNCHRONIZER_ID" \
      --argjson inputHoldingCids "$MATCHING_HOLDING_CIDS" \
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
                                        { label: "id", value: { text: $symbol } }
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
            readAs: [$sender],
            submissionId: $submissionId,
            synchronizerId: $synchronizerId
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
              ($receiver): {
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
      }'
  else
    jq -n \
      --arg transferPackageId "$TRANSFER_PACKAGE_ID" \
      --arg factoryCid "$FACTORY_CONTRACT_ID" \
      --arg sender "$SENDER_PARTY" \
      --arg receiver "$RECEIVER_PARTY" \
      --arg admin "$ADMIN_PARTY" \
      --arg symbol "$SYMBOL" \
      --arg amount "$AMOUNT" \
      --arg requestedAt "$REQUESTED_AT_RFC3339" \
      --arg executeBefore "$EXECUTE_BEFORE_RFC3339" \
      --arg userId "${USER_ID:-}" \
      --arg commandId "$COMMAND_ID" \
      --arg submissionId "$SUBMISSION_ID" \
      --arg dedupDuration "${DEDUP_SECONDS}s" \
      --argjson inputHoldingCids "$MATCHING_HOLDING_CIDS" \
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
                                        { label: "id", value: { text: $symbol } }
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
            readAs: [$sender],
            submissionId: $submissionId
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
              ($receiver): {
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
      }'
  fi
}

build_payload | canton_grpc_call "com.daml.ledger.api.v2.CommandService/SubmitAndWaitForTransaction"
