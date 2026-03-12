#!/usr/bin/env bash

set -euo pipefail

SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/lib/canton_token_standard.sh"

usage() {
  cat <<'EOF'
Accept a pending simple-token transfer instruction.

Required environment variables:
  PARTICIPANT_HOST          gRPC host:port of the participant
  AUTH_TOKEN                Bearer token value, unless AUTH_HEADER is provided
  PARTY_ID                  Receiver / controller party accepting the instruction
  TRANSFER_INSTRUCTION_ID   Contract id of the SimpleTokenTransferInstruction

Optional environment variables:
  AUTH_HEADER               Full authorization header value
  USER_ID                   Optional Ledger API user id
  COMMAND_ID                Defaults to simple-token-accept-<timestamp>
  SUBMISSION_ID             Defaults to accept-simple-token-<timestamp>
  DEDUP_SECONDS             Defaults to 300
  TRANSFER_PACKAGE          Defaults to splice-api-token-transfer-instruction-v1
  SYNCHRONIZER_ID           Optional synchronizer/domain id
EOF
}

if [[ "${1:-}" == "-h" || "${1:-}" == "--help" ]]; then
  usage
  exit 0
fi

: "${PARTICIPANT_HOST:?PARTICIPANT_HOST is required}"
: "${PARTY_ID:?PARTY_ID is required}"
: "${TRANSFER_INSTRUCTION_ID:?TRANSFER_INSTRUCTION_ID is required}"

COMMAND_ID="${COMMAND_ID:-simple-token-accept-$(date +%s)}"
SUBMISSION_ID="${SUBMISSION_ID:-accept-simple-token-$(date +%s)}"
DEDUP_SECONDS="${DEDUP_SECONDS:-300}"
TRANSFER_PACKAGE="${TRANSFER_PACKAGE:-splice-api-token-transfer-instruction-v1}"

TRANSFER_PACKAGE_ID="$(canton_resolve_package_id_by_name "$TRANSFER_PACKAGE")"
if [[ -z "$TRANSFER_PACKAGE_ID" ]]; then
  echo "Could not resolve package id for $TRANSFER_PACKAGE" >&2
  exit 1
fi

build_payload() {
  if [[ -n "${SYNCHRONIZER_ID:-}" ]]; then
    jq -n \
      --arg packageId "$TRANSFER_PACKAGE_ID" \
      --arg party "$PARTY_ID" \
      --arg instructionId "$TRANSFER_INSTRUCTION_ID" \
      --arg userId "${USER_ID:-}" \
      --arg commandId "$COMMAND_ID" \
      --arg submissionId "$SUBMISSION_ID" \
      --arg dedupDuration "${DEDUP_SECONDS}s" \
      --arg synchronizerId "$SYNCHRONIZER_ID" \
      '{
        commands: (
          {
            commandId: $commandId,
            commands: [
              {
                exercise: {
                  templateId: {
                    packageId: $packageId,
                    moduleName: "Splice.Api.Token.TransferInstructionV1",
                    entityName: "TransferInstruction"
                  },
                  contractId: $instructionId,
                  choice: "TransferInstruction_Accept",
                  choiceArgument: {
                    record: {
                      fields: [
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
            actAs: [$party],
            readAs: [$party],
            submissionId: $submissionId,
            synchronizerId: $synchronizerId
          }
          | if $userId != "" then . + { userId: $userId } else . end
        ),
        transactionFormat: {
          eventFormat: {
            filtersByParty: {
              ($party): {
                cumulative: [
                  {
                    interfaceFilter: {
                      interfaceId: {
                        packageId: $packageId,
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
      --arg packageId "$TRANSFER_PACKAGE_ID" \
      --arg party "$PARTY_ID" \
      --arg instructionId "$TRANSFER_INSTRUCTION_ID" \
      --arg userId "${USER_ID:-}" \
      --arg commandId "$COMMAND_ID" \
      --arg submissionId "$SUBMISSION_ID" \
      --arg dedupDuration "${DEDUP_SECONDS}s" \
      '{
        commands: (
          {
            commandId: $commandId,
            commands: [
              {
                exercise: {
                  templateId: {
                    packageId: $packageId,
                    moduleName: "Splice.Api.Token.TransferInstructionV1",
                    entityName: "TransferInstruction"
                  },
                  contractId: $instructionId,
                  choice: "TransferInstruction_Accept",
                  choiceArgument: {
                    record: {
                      fields: [
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
            actAs: [$party],
            readAs: [$party],
            submissionId: $submissionId
          }
          | if $userId != "" then . + { userId: $userId } else . end
        ),
        transactionFormat: {
          eventFormat: {
            filtersByParty: {
              ($party): {
                cumulative: [
                  {
                    interfaceFilter: {
                      interfaceId: {
                        packageId: $packageId,
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
