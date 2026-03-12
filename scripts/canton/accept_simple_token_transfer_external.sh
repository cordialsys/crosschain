#!/usr/bin/env bash

set -euo pipefail

SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/lib/canton_token_standard.sh"

usage() {
  cat <<'EOF'
Accept a pending simple-token transfer instruction as an external Canton party.

This uses Canton interactive submission:
1. PrepareSubmission as the external party
2. Sign the prepared transaction hash with the external Ed25519 key
3. ExecuteSubmissionAndWait with the external party signature

Required environment variables:
  PARTICIPANT_HOST          gRPC host:port of the participant
  AUTH_TOKEN                Bearer token value, unless AUTH_HEADER is provided
  PARTY_ID                  External receiver party accepting the instruction
  TRANSFER_INSTRUCTION_ID   Contract id of the SimpleTokenTransferInstruction
  PRIVATE_KEY_HEX           Ed25519 private key hex (32-byte seed or 64-byte private key)

Optional environment variables:
  AUTH_HEADER               Full authorization header value
  USER_ID                   Optional Ledger API user id
  COMMAND_ID                Defaults to simple-token-accept-external-<timestamp>
  SUBMISSION_ID             Defaults to accept-simple-token-external-<timestamp>
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
: "${PRIVATE_KEY_HEX:?PRIVATE_KEY_HEX is required}"

COMMAND_ID="${COMMAND_ID:-simple-token-accept-external-$(date +%s)}"
SUBMISSION_ID="${SUBMISSION_ID:-accept-simple-token-external-$(date +%s)}"
DEDUP_SECONDS="${DEDUP_SECONDS:-300}"
TRANSFER_PACKAGE="${TRANSFER_PACKAGE:-splice-api-token-transfer-instruction-v1}"

TRANSFER_PACKAGE_ID="$(canton_resolve_package_id_by_name "$TRANSFER_PACKAGE")"
if [[ -z "$TRANSFER_PACKAGE_ID" ]]; then
  echo "Could not resolve package id for $TRANSFER_PACKAGE" >&2
  exit 1
fi

PARTY_FINGERPRINT="$(canton_party_fingerprint "$PARTY_ID")"

build_prepare_payload() {
  if [[ -n "${SYNCHRONIZER_ID:-}" ]]; then
    jq -n \
      --arg packageId "$TRANSFER_PACKAGE_ID" \
      --arg party "$PARTY_ID" \
      --arg instructionId "$TRANSFER_INSTRUCTION_ID" \
      --arg userId "${USER_ID:-}" \
      --arg commandId "$COMMAND_ID" \
      --arg synchronizerId "$SYNCHRONIZER_ID" \
      '(
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
          actAs: [$party],
          readAs: [$party],
          verboseHashing: false,
          synchronizerId: $synchronizerId
        }
        | if $userId != "" then . + { userId: $userId } else . end
      )'
  else
    jq -n \
      --arg packageId "$TRANSFER_PACKAGE_ID" \
      --arg party "$PARTY_ID" \
      --arg instructionId "$TRANSFER_INSTRUCTION_ID" \
      --arg userId "${USER_ID:-}" \
      --arg commandId "$COMMAND_ID" \
      '(
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
          actAs: [$party],
          readAs: [$party],
          verboseHashing: false
        }
        | if $userId != "" then . + { userId: $userId } else . end
      )'
  fi
}

PREPARE_RESPONSE="$(build_prepare_payload | canton_grpc_call "com.daml.ledger.api.v2.interactive.InteractiveSubmissionService/PrepareSubmission")"

PREPARED_TX_JSON="$(jq -c '(.prepared_transaction // .preparedTransaction // empty)' <<<"$PREPARE_RESPONSE")"
PREPARED_HASH_B64="$(jq -r '(.prepared_transaction_hash // .preparedTransactionHash // empty)' <<<"$PREPARE_RESPONSE")"
HASHING_SCHEME_VERSION="$(jq -r '(.hashing_scheme_version // .hashingSchemeVersion // empty)' <<<"$PREPARE_RESPONSE")"

if [[ -z "$PREPARED_TX_JSON" || "$PREPARED_TX_JSON" == "null" ]]; then
  echo "PrepareSubmission response missing prepared_transaction:" >&2
  echo "$PREPARE_RESPONSE" >&2
  exit 1
fi
if [[ -z "$PREPARED_HASH_B64" ]]; then
  echo "PrepareSubmission response missing prepared_transaction_hash:" >&2
  echo "$PREPARE_RESPONSE" >&2
  exit 1
fi
if [[ -z "$HASHING_SCHEME_VERSION" ]]; then
  echo "PrepareSubmission response missing hashing_scheme_version:" >&2
  echo "$PREPARE_RESPONSE" >&2
  exit 1
fi

SIGNATURE_JSON="$(canton_ed25519_sign_prepared_hash_json "$PARTY_ID" "$PRIVATE_KEY_HEX" "$PREPARED_HASH_B64")"
SIGNATURE_B64="$(jq -r '.signature_b64' <<<"$SIGNATURE_JSON")"

build_execute_payload() {
  jq -n \
    --argjson preparedTransaction "$PREPARED_TX_JSON" \
    --arg party "$PARTY_ID" \
    --arg signatureB64 "$SIGNATURE_B64" \
    --arg signedBy "$PARTY_FINGERPRINT" \
    --arg submissionId "$SUBMISSION_ID" \
    --arg userId "${USER_ID:-}" \
    --arg hashingSchemeVersion "$HASHING_SCHEME_VERSION" \
    --arg dedupDuration "${DEDUP_SECONDS}s" \
    '(
      {
        preparedTransaction: $preparedTransaction,
        partySignatures: {
          signatures: [
            {
              party: $party,
              signatures: [
                {
                  format: "SIGNATURE_FORMAT_RAW",
                  signature: $signatureB64,
                  signedBy: $signedBy,
                  signingAlgorithmSpec: "SIGNING_ALGORITHM_SPEC_ED25519"
                }
              ]
            }
          ]
        },
        deduplicationDuration: $dedupDuration,
        submissionId: $submissionId,
        hashingSchemeVersion: $hashingSchemeVersion
      }
      | if $userId != "" then . + { userId: $userId } else . end
    )'
}

build_execute_payload | canton_grpc_call "com.daml.ledger.api.v2.interactive.InteractiveSubmissionService/ExecuteSubmissionAndWait"
