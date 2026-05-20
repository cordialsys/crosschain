#!/usr/bin/env bash

set -euo pipefail

SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/lib/canton_token_standard.sh"

usage() {
  cat <<'EOF'
Create a SimpleTokenFactory contract on a Canton participant using grpcurl.

Required environment variables:
  PARTICIPANT_HOST   gRPC host:port of the participant
  AUTH_TOKEN         Bearer token value, unless AUTH_HEADER is provided
  ADMIN_PARTY        Admin / issuer party for the token
  SYMBOL             Instrument id text, for example "TEST"

Optional environment variables:
  AUTH_HEADER        Full authorization header value
  USER_ID            Optional Ledger API user id
  COMMAND_ID         Defaults to simple-token-factory-<timestamp>
  SUBMISSION_ID      Defaults to create-simple-token-factory-<timestamp>
  DEDUP_SECONDS      Defaults to 300
  SYNCHRONIZER_ID    Optional synchronizer/domain id
  PACKAGE_NAME       Defaults to splice-token-test-simple-transfer
EOF
}

if [[ "${1:-}" == "-h" || "${1:-}" == "--help" ]]; then
  usage
  exit 0
fi

: "${PARTICIPANT_HOST:?PARTICIPANT_HOST is required}"
: "${ADMIN_PARTY:?ADMIN_PARTY is required}"
: "${SYMBOL:?SYMBOL is required}"

COMMAND_ID="${COMMAND_ID:-simple-token-factory-$(date +%s)}"
SUBMISSION_ID="${SUBMISSION_ID:-create-simple-token-factory-$(date +%s)}"
DEDUP_SECONDS="${DEDUP_SECONDS:-300}"
PACKAGE_NAME="${PACKAGE_NAME:-splice-token-test-simple-transfer}"

PACKAGE_ID="$(canton_resolve_package_id_by_name "$PACKAGE_NAME")"
if [[ -z "$PACKAGE_ID" ]]; then
  echo "Could not resolve package id for $PACKAGE_NAME. Upload the DAR first." >&2
  exit 1
fi

if [[ -n "${SYNCHRONIZER_ID:-}" ]]; then
  jq -n \
    --arg packageId "$PACKAGE_ID" \
    --arg admin "$ADMIN_PARTY" \
    --arg symbol "$SYMBOL" \
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
              create: {
                templateId: {
                  packageId: $packageId,
                  moduleName: "Splice.Api.Token.Test.SimpleTransferToken",
                  entityName: "SimpleTokenFactory"
                },
                createArguments: {
                  recordId: {
                    packageId: $packageId,
                    moduleName: "Splice.Api.Token.Test.SimpleTransferToken",
                    entityName: "SimpleTokenFactory"
                  },
                  fields: [
                    { label: "admin", value: { party: $admin } },
                    { label: "symbol", value: { text: $symbol } }
                  ]
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
            ($admin): {
              cumulative: [
                {
                  templateFilter: {
                    templateId: {
                      packageId: $packageId,
                      moduleName: "Splice.Api.Token.Test.SimpleTransferToken",
                      entityName: "SimpleTokenFactory"
                    },
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
else
  jq -n \
    --arg packageId "$PACKAGE_ID" \
    --arg admin "$ADMIN_PARTY" \
    --arg symbol "$SYMBOL" \
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
              create: {
                templateId: {
                  packageId: $packageId,
                  moduleName: "Splice.Api.Token.Test.SimpleTransferToken",
                  entityName: "SimpleTokenFactory"
                },
                createArguments: {
                  recordId: {
                    packageId: $packageId,
                    moduleName: "Splice.Api.Token.Test.SimpleTransferToken",
                    entityName: "SimpleTokenFactory"
                  },
                  fields: [
                    { label: "admin", value: { party: $admin } },
                    { label: "symbol", value: { text: $symbol } }
                  ]
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
            ($admin): {
              cumulative: [
                {
                  templateFilter: {
                    templateId: {
                      packageId: $packageId,
                      moduleName: "Splice.Api.Token.Test.SimpleTransferToken",
                      entityName: "SimpleTokenFactory"
                    },
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
fi
