#!/usr/bin/env bash

set -euo pipefail

SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/lib/canton_token_standard.sh"

usage() {
  cat <<'EOF'
Mint a SimpleTokenHolding by exercising SimpleTokenFactory_Mint.

Required environment variables:
  PARTICIPANT_HOST      gRPC host:port of the participant
  AUTH_TOKEN            Bearer token value, unless AUTH_HEADER is provided
  ADMIN_PARTY           Admin / issuer party controlling the factory
  FACTORY_CONTRACT_ID   Contract id of the SimpleTokenFactory
  OWNER_PARTY           Party that will own the holding
  AMOUNT                Decimal amount, for example 10.0

Optional environment variables:
  AUTH_HEADER           Full authorization header value
  USER_ID               Optional Ledger API user id
  COMMAND_ID            Defaults to simple-token-mint-<timestamp>
  SUBMISSION_ID         Defaults to mint-simple-token-<timestamp>
  DEDUP_SECONDS         Defaults to 300
  PACKAGE_NAME          Defaults to splice-token-test-simple-transfer
  HOLDING_PACKAGE       Defaults to splice-api-token-holding-v1
  SYNCHRONIZER_ID       Optional synchronizer/domain id
EOF
}

if [[ "${1:-}" == "-h" || "${1:-}" == "--help" ]]; then
  usage
  exit 0
fi

: "${PARTICIPANT_HOST:?PARTICIPANT_HOST is required}"
: "${ADMIN_PARTY:?ADMIN_PARTY is required}"
: "${FACTORY_CONTRACT_ID:?FACTORY_CONTRACT_ID is required}"
: "${OWNER_PARTY:?OWNER_PARTY is required}"
: "${AMOUNT:?AMOUNT is required}"

COMMAND_ID="${COMMAND_ID:-simple-token-mint-$(date +%s)}"
SUBMISSION_ID="${SUBMISSION_ID:-mint-simple-token-$(date +%s)}"
DEDUP_SECONDS="${DEDUP_SECONDS:-300}"
PACKAGE_NAME="${PACKAGE_NAME:-splice-token-test-simple-transfer}"
HOLDING_PACKAGE="${HOLDING_PACKAGE:-splice-api-token-holding-v1}"

PACKAGE_ID="$(canton_resolve_package_id_by_name "$PACKAGE_NAME")"
if [[ -z "$PACKAGE_ID" ]]; then
  echo "Could not resolve package id for $PACKAGE_NAME" >&2
  exit 1
fi

HOLDING_PACKAGE_ID="$(canton_resolve_package_id_by_name "$HOLDING_PACKAGE")"
if [[ -z "$HOLDING_PACKAGE_ID" ]]; then
  echo "Could not resolve package id for $HOLDING_PACKAGE" >&2
  exit 1
fi

build_payload() {
  if [[ -n "${SYNCHRONIZER_ID:-}" ]]; then
    jq -n \
      --arg packageId "$PACKAGE_ID" \
      --arg holdingPackageId "$HOLDING_PACKAGE_ID" \
      --arg admin "$ADMIN_PARTY" \
      --arg factoryCid "$FACTORY_CONTRACT_ID" \
      --arg owner "$OWNER_PARTY" \
      --arg amount "$AMOUNT" \
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
                    moduleName: "Splice.Api.Token.Test.SimpleTransferToken",
                    entityName: "SimpleTokenFactory"
                  },
                  contractId: $factoryCid,
                  choice: "SimpleTokenFactory_Mint",
                  choiceArgument: {
                    record: {
                      fields: [
                        { label: "owner", value: { party: $owner } },
                        { label: "amount", value: { numeric: $amount } }
                      ]
                    }
                  }
                }
              }
            ],
            deduplicationDuration: $dedupDuration,
            actAs: [$admin],
            readAs: [$admin],
            submissionId: $submissionId,
            synchronizerId: $synchronizerId
          }
          | if $userId != "" then . + { userId: $userId } else . end
        ),
        transactionFormat: {
          eventFormat: {
            filtersByParty: {
              ($owner): {
                cumulative: [
                  {
                    interfaceFilter: {
                      interfaceId: {
                        packageId: $holdingPackageId,
                        moduleName: "Splice.Api.Token.HoldingV1",
                        entityName: "Holding"
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
      --arg packageId "$PACKAGE_ID" \
      --arg holdingPackageId "$HOLDING_PACKAGE_ID" \
      --arg admin "$ADMIN_PARTY" \
      --arg factoryCid "$FACTORY_CONTRACT_ID" \
      --arg owner "$OWNER_PARTY" \
      --arg amount "$AMOUNT" \
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
                    moduleName: "Splice.Api.Token.Test.SimpleTransferToken",
                    entityName: "SimpleTokenFactory"
                  },
                  contractId: $factoryCid,
                  choice: "SimpleTokenFactory_Mint",
                  choiceArgument: {
                    record: {
                      fields: [
                        { label: "owner", value: { party: $owner } },
                        { label: "amount", value: { numeric: $amount } }
                      ]
                    }
                  }
                }
              }
            ],
            deduplicationDuration: $dedupDuration,
            actAs: [$admin],
            readAs: [$admin],
            submissionId: $submissionId
          }
          | if $userId != "" then . + { userId: $userId } else . end
        ),
        transactionFormat: {
          eventFormat: {
            filtersByParty: {
              ($owner): {
                cumulative: [
                  {
                    interfaceFilter: {
                      interfaceId: {
                        packageId: $holdingPackageId,
                        moduleName: "Splice.Api.Token.HoldingV1",
                        entityName: "Holding"
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
