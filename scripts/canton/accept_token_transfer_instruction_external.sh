#!/usr/bin/env bash

set -euo pipefail

SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/lib/canton_token_standard.sh"

usage() {
  cat <<'EOF'
Accept a token-standard TransferInstruction as an external Canton party.

This uses Canton interactive submission:
1. Fetch registry accept choice-context/disclosures
2. PrepareSubmission as the external party
3. Sign the prepared transaction hash with the external Ed25519 key
4. ExecuteSubmissionAndWait with the external party signature

Required environment variables:
  PARTICIPANT_HOST          gRPC host:port of the participant
  AUTH_TOKEN                Bearer token value, unless AUTH_HEADER is provided
  PARTY_ID                  External receiver / controller party
  TRANSFER_INSTRUCTION_ID   Contract id of the TransferInstruction
  PRIVATE_KEY_HEX           Ed25519 private key hex (32-byte seed or 64-byte private key)

Registry location:
  Either set SCAN_PROXY_URL + SCAN_API_URL
  or set REGISTRY_BASE_URL for direct access.

Registry auth:
  SCAN_AUTH_TOKEN           Optional for direct public registries, required for proxy/private access
  SCAN_AUTH_HEADER          Full registry authorization header

Optional environment variables:
  AUTH_HEADER               Full ledger authorization header value
  USER_ID                   Optional Ledger API user id
  COMMAND_ID                Defaults to ts-accept-external-<timestamp>
  SUBMISSION_ID             Defaults to ts-accept-external-<timestamp>
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
: "${PRIVATE_KEY_HEX:?PRIVATE_KEY_HEX is required}"

PACKAGE_NAME="${PACKAGE_NAME:-splice-api-token-transfer-instruction-v1}"
COMMAND_ID="${COMMAND_ID:-ts-accept-external-$(date +%s)}"
SUBMISSION_ID="${SUBMISSION_ID:-ts-accept-external-$(date +%s)}"
DEDUP_SECONDS="${DEDUP_SECONDS:-300}"

PACKAGE_MAP_JSON="$(canton_build_package_map_json)"
TRANSFER_PACKAGE_ID="$(canton_resolve_package_id_by_name "$PACKAGE_NAME")"
if [[ -z "$TRANSFER_PACKAGE_ID" ]]; then
  echo "Could not resolve package id for $PACKAGE_NAME" >&2
  exit 1
fi

REGISTRY_RESPONSE="$(canton_scan_api_post "/registry/transfer-instruction/v1/${TRANSFER_INSTRUCTION_ID}/choice-contexts/accept" '{}')"

if ! jq -e '.choiceContextData and .disclosedContracts' >/dev/null <<<"$REGISTRY_RESPONSE"; then
  echo "Unexpected accept-context registry response:" >&2
  echo "$REGISTRY_RESPONSE" >&2
  exit 1
fi

PARTY_FINGERPRINT="$(canton_party_fingerprint "$PARTY_ID")"

PREPARE_RESPONSE="$(
  jq -n \
    --arg party "$PARTY_ID" \
    --arg commandId "$COMMAND_ID" \
    --arg instructionId "$TRANSFER_INSTRUCTION_ID" \
    --arg transferPackageId "$TRANSFER_PACKAGE_ID" \
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
    (
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
        actAs: [$party],
        readAs: [$party],
        verboseHashing: false,
        synchronizerId: ($registry.disclosedContracts[0].synchronizerId)
      }
      | if $userId != "" then . + { userId: $userId } else . end
    )' \
  | canton_grpc_call "com.daml.ledger.api.v2.interactive.InteractiveSubmissionService/PrepareSubmission"
)"

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
  )' \
  | canton_grpc_call "com.daml.ledger.api.v2.interactive.InteractiveSubmissionService/ExecuteSubmissionAndWait"
