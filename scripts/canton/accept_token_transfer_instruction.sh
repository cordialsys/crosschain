#!/usr/bin/env bash

set -euo pipefail

SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/lib/canton_token_standard.sh"

usage() {
  cat <<'EOF'
Accept a token-standard TransferInstruction for an internal Canton party using grpcurl.

Required environment variables:
  PARTICIPANT_HOST          gRPC host:port of the participant
  AUTH_TOKEN                Bearer token value, unless AUTH_HEADER is provided
  PARTY_ID                  Receiver / controller party accepting the instruction
  TRANSFER_INSTRUCTION_ID   Contract id of the TransferInstruction

Registry auth:
  SCAN_AUTH_TOKEN           Bearer token for the scan registry/proxy, unless SCAN_AUTH_HEADER is provided

Registry location:
  Either set SCAN_PROXY_URL + SCAN_API_URL
  or set REGISTRY_BASE_URL for direct access.

Optional environment variables:
  AUTH_HEADER               Full ledger authorization header
  SCAN_AUTH_HEADER          Full registry authorization header
  USER_ID                   Optional Ledger API user id
  COMMAND_ID                Defaults to ts-accept-<timestamp>
  SUBMISSION_ID             Defaults to ts-accept-<timestamp>
  DEDUP_SECONDS             Defaults to 300
  PACKAGE_NAME              Defaults to splice-api-token-transfer-instruction-v1
EOF
}

if [[ "${1:-}" == "-h" || "${1:-}" == "--help" ]]; then
  usage
  exit 0
fi

: "${PARTICIPANT_HOST:?PARTICIPANT_HOST is required}"
: "${PARTY_ID:?PARTY_ID is required}"
: "${TRANSFER_INSTRUCTION_ID:?TRANSFER_INSTRUCTION_ID is required}"

PACKAGE_NAME="${PACKAGE_NAME:-splice-api-token-transfer-instruction-v1}"
COMMAND_ID="${COMMAND_ID:-ts-accept-$(date +%s)}"
SUBMISSION_ID="${SUBMISSION_ID:-ts-accept-$(date +%s)}"
DEDUP_SECONDS="${DEDUP_SECONDS:-300}"

PACKAGE_MAP_JSON="$(canton_build_package_map_json)"
TRANSFER_PACKAGE_ID="$(canton_resolve_package_id_by_name "$PACKAGE_NAME")"
if [[ -z "$TRANSFER_PACKAGE_ID" ]]; then
  echo "Could not resolve package id for $PACKAGE_NAME" >&2
  exit 1
fi

REGISTRY_REQUEST='{}'
REGISTRY_RESPONSE="$(canton_scan_api_post "/registry/transfer-instruction/v1/${TRANSFER_INSTRUCTION_ID}/choice-contexts/accept" "$REGISTRY_REQUEST")"

if ! jq -e '.choiceContextData and .disclosedContracts' >/dev/null <<<"$REGISTRY_RESPONSE"; then
  echo "Unexpected accept-context registry response:" >&2
  echo "$REGISTRY_RESPONSE" >&2
  exit 1
fi

jq -n \
  --arg party "$PARTY_ID" \
  --arg commandId "$COMMAND_ID" \
  --arg submissionId "$SUBMISSION_ID" \
  --arg dedupDuration "${DEDUP_SECONDS}s" \
  --arg transferPackageId "$TRANSFER_PACKAGE_ID" \
  --arg instructionId "$TRANSFER_INSTRUCTION_ID" \
  --arg userId "${USER_ID:-}" \
  --argjson packageMap "$PACKAGE_MAP_JSON" \
  --argjson registry "$REGISTRY_RESPONSE" \
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
                            { label: "context", value: choice_context_to_value($registry.choiceContextData) },
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
        disclosedContracts: ($registry.disclosedContracts | map({
          templateId: (.templateId | template_ref_to_identifier($packageMap)),
          contractId: .contractId,
          createdEventBlob: .createdEventBlob,
          synchronizerId: .synchronizerId
        })),
        deduplicationDuration: $dedupDuration,
        actAs: [$party],
        readAs: [$party],
        submissionId: $submissionId,
        synchronizerId: ($registry.disclosedContracts[0].synchronizerId)
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
