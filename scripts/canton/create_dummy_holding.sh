#!/usr/bin/env bash

set -euo pipefail

usage() {
  cat <<'EOF'
Create a live DummyHolding token contract on a Canton participant using grpcurl.

This targets the example token implementation package:
  splice-token-test-dummy-holding

This example is useful for balance/read-path testing only.
It does not implement TransferFactory / TransferInstruction, so it is not a
transfer-capable token-standard implementation.

Required environment variables:
  PARTICIPANT_HOST   gRPC host:port of the participant
  AUTH_TOKEN         Bearer token value, unless AUTH_HEADER is provided
  OWNER_PARTY        Party that will own the holding
  AMOUNT             Decimal amount, for example 10.0

Optional environment variables:
  AUTH_HEADER        Full authorization header value, e.g. "Bearer ey..." or "Basic abc..."
  ISSUER_PARTY       Defaults to OWNER_PARTY
  USER_ID            Optional Ledger API user id. Omitted if unset.
  COMMAND_ID         Defaults to dummy-holding-<timestamp>
  SUBMISSION_ID      Defaults to create-dummy-holding-<timestamp>
  DEDUP_SECONDS      Defaults to 300
  SYNCHRONIZER_ID    Optional synchronizer/domain id
  PACKAGE_NAME       Defaults to splice-token-test-dummy-holding

Notes:
  - DummyHolding has signatories owner and issuer.
  - The auth token must be authorized to act_as both parties.
  - The script uses gRPC reflection and auto-resolves the uploaded package_id from ListKnownPackages.
EOF
}

if [[ "${1:-}" == "-h" || "${1:-}" == "--help" ]]; then
  usage
  exit 0
fi

: "${PARTICIPANT_HOST:?PARTICIPANT_HOST is required}"
: "${AUTH_TOKEN:?AUTH_TOKEN is required}"
: "${OWNER_PARTY:?OWNER_PARTY is required}"
: "${AMOUNT:?AMOUNT is required}"

AUTH_HEADER="${AUTH_HEADER:-Bearer $AUTH_TOKEN}"
ISSUER_PARTY="${ISSUER_PARTY:-$OWNER_PARTY}"
USER_ID="${USER_ID:-}"
COMMAND_ID="${COMMAND_ID:-dummy-holding-$(date +%s)}"
SUBMISSION_ID="${SUBMISSION_ID:-create-dummy-holding-$(date +%s)}"
DEDUP_SECONDS="${DEDUP_SECONDS:-300}"
PACKAGE_NAME="${PACKAGE_NAME:-splice-token-test-dummy-holding}"

grpc_call() {
  local method="$1"
  local tmp
  tmp="$(mktemp)"
  if ! grpcurl \
    -H "authorization: $AUTH_HEADER" \
    -d @ \
    "$PARTICIPANT_HOST" \
    "$method" >"$tmp"; then
    cat "$tmp" >&2 || true
    rm -f "$tmp"
    return 1
  fi
  cat "$tmp"
  rm -f "$tmp"
}

resolve_package_id() {
  local response
  response="$(printf '{}' | grpc_call "com.daml.ledger.api.v2.admin.PackageManagementService/ListKnownPackages")" || return 1
  if ! jq -e '(.package_details // .packageDetails) != null' >/dev/null <<<"$response"; then
    echo "Unexpected ListKnownPackages response:" >&2
    echo "$response" >&2
    return 1
  fi
  jq -r --arg packageName "$PACKAGE_NAME" '
        (.package_details // .packageDetails)
        | map(select(.name == $packageName))
        | sort_by((.known_since // .knownSince))
        | last
        | (.package_id // .packageId // empty)
      ' <<<"$response"
}

PACKAGE_ID="$(resolve_package_id)"
if [[ -z "$PACKAGE_ID" ]]; then
  echo "Could not resolve package id for $PACKAGE_NAME. Upload the implementation DAR first." >&2
  exit 1
fi

build_request() {
  if [[ -n "${SYNCHRONIZER_ID:-}" ]]; then
    jq -n \
      --arg packageId "$PACKAGE_ID" \
      --arg owner "$OWNER_PARTY" \
      --arg issuer "$ISSUER_PARTY" \
      --arg amount "$AMOUNT" \
      --arg userId "$USER_ID" \
      --arg commandId "$COMMAND_ID" \
      --arg submissionId "$SUBMISSION_ID" \
      --arg dedupDuration "${DEDUP_SECONDS}s" \
      --arg synchronizerId "$SYNCHRONIZER_ID" \
      '
      {
        commands: (
          {
            commandId: $commandId,
            commands: [
              {
                create: {
                  templateId: {
                    packageId: $packageId,
                    moduleName: "Splice.Api.Token.Test.DummyHolding",
                    entityName: "DummyHolding"
                  },
                  createArguments: {
                    recordId: {
                      packageId: $packageId,
                      moduleName: "Splice.Api.Token.Test.DummyHolding",
                      entityName: "DummyHolding"
                    },
                    fields: [
                      { label: "owner", value: { party: $owner } },
                      { label: "issuer", value: { party: $issuer } },
                      { label: "amount", value: { numeric: $amount } }
                    ]
                  }
                }
              }
            ],
            deduplicationDuration: $dedupDuration,
            actAs: [$owner, $issuer],
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
                    templateFilter: {
                      templateId: {
                        packageId: $packageId,
                        moduleName: "Splice.Api.Token.Test.DummyHolding",
                        entityName: "DummyHolding"
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
      }'
  else
    jq -n \
      --arg packageId "$PACKAGE_ID" \
      --arg owner "$OWNER_PARTY" \
      --arg issuer "$ISSUER_PARTY" \
      --arg amount "$AMOUNT" \
      --arg userId "$USER_ID" \
      --arg commandId "$COMMAND_ID" \
      --arg submissionId "$SUBMISSION_ID" \
      --arg dedupDuration "${DEDUP_SECONDS}s" '
      {
        commands: (
          {
            commandId: $commandId,
            commands: [
              {
                create: {
                  templateId: {
                    packageId: $packageId,
                    moduleName: "Splice.Api.Token.Test.DummyHolding",
                    entityName: "DummyHolding"
                  },
                  createArguments: {
                    recordId: {
                      packageId: $packageId,
                      moduleName: "Splice.Api.Token.Test.DummyHolding",
                      entityName: "DummyHolding"
                    },
                    fields: [
                      { label: "owner", value: { party: $owner } },
                      { label: "issuer", value: { party: $issuer } },
                      { label: "amount", value: { numeric: $amount } }
                    ]
                  }
                }
              }
            ],
            deduplicationDuration: $dedupDuration,
            actAs: [$owner, $issuer],
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
                    templateFilter: {
                      templateId: {
                        packageId: $packageId,
                        moduleName: "Splice.Api.Token.Test.DummyHolding",
                        entityName: "DummyHolding"
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
      }'
  fi
}

echo "Resolved package id: $PACKAGE_ID" >&2
build_request \
  | grpc_call "com.daml.ledger.api.v2.CommandService/SubmitAndWaitForTransaction"
