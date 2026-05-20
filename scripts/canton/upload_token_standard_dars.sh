#!/usr/bin/env bash

set -euo pipefail

usage() {
  cat <<'EOF'
Upload Canton token-standard interface DARs to a participant using grpcurl.

Required environment variables:
  PARTICIPANT_HOST   gRPC host:port of the participant
  AUTH_TOKEN         Bearer token with package upload rights

Optional environment variables:
  SYNCHRONIZER_ID    Target synchronizer/domain id for vetting
  PROTO_ROOT         Defaults to ~/Source/splice/canton/community/ledger-api/src/main/protobuf
  PACKAGE_PROTO      Defaults to com/daml/ledger/api/v2/admin/package_management_service.proto
  VALIDATE_FIRST     Defaults to 1. Set to 0 to skip ValidateDarFile calls.
  LIST_AFTER         Defaults to 1. Set to 0 to skip ListKnownPackages at the end.

The script uploads these DARs in dependency order:
  splice-api-token-metadata-v1
  splice-api-token-holding-v1
  splice-api-token-transfer-instruction-v1
  splice-api-token-allocation-v1
  splice-api-token-allocation-request-v1
  splice-api-token-allocation-instruction-v1
EOF
}

if [[ "${1:-}" == "-h" || "${1:-}" == "--help" ]]; then
  usage
  exit 0
fi

: "${PARTICIPANT_HOST:?PARTICIPANT_HOST is required}"
: "${AUTH_TOKEN:?AUTH_TOKEN is required}"

PROTO_ROOT="${PROTO_ROOT:-$HOME/Source/splice/canton/community/ledger-api/src/main/protobuf}"
PACKAGE_PROTO="${PACKAGE_PROTO:-com/daml/ledger/api/v2/admin/package_management_service.proto}"
VALIDATE_FIRST="${VALIDATE_FIRST:-1}"
LIST_AFTER="${LIST_AFTER:-1}"

DAR_METADATA="${DAR_METADATA:-$HOME/Source/splice/token-standard/splice-api-token-metadata-v1/.daml/dist/splice-api-token-metadata-v1-1.0.0.dar}"
DAR_HOLDING="${DAR_HOLDING:-$HOME/Source/splice/token-standard/splice-api-token-holding-v1/.daml/dist/splice-api-token-holding-v1-1.0.0.dar}"
DAR_TRANSFER_INSTR="${DAR_TRANSFER_INSTR:-$HOME/Source/splice/token-standard/splice-api-token-transfer-instruction-v1/.daml/dist/splice-api-token-transfer-instruction-v1-1.0.0.dar}"
DAR_ALLOCATION="${DAR_ALLOCATION:-$HOME/Source/splice/token-standard/splice-api-token-allocation-v1/.daml/dist/splice-api-token-allocation-v1-1.0.0.dar}"
DAR_ALLOC_REQ="${DAR_ALLOC_REQ:-$HOME/Source/splice/token-standard/splice-api-token-allocation-request-v1/.daml/dist/splice-api-token-allocation-request-v1-1.0.0.dar}"
DAR_ALLOC_INSTR="${DAR_ALLOC_INSTR:-$HOME/Source/splice/token-standard/splice-api-token-allocation-instruction-v1/.daml/dist/splice-api-token-allocation-instruction-v1-1.0.0.dar}"

DARS=(
  "$DAR_METADATA"
  "$DAR_HOLDING"
  "$DAR_TRANSFER_INSTR"
  "$DAR_ALLOCATION"
  "$DAR_ALLOC_REQ"
  "$DAR_ALLOC_INSTR"
)

for dar in "${DARS[@]}"; do
  if [[ ! -f "$dar" ]]; then
    echo "DAR not found: $dar" >&2
    exit 1
  fi
done

if [[ ! -f "$PROTO_ROOT/$PACKAGE_PROTO" ]]; then
  echo "Proto not found: $PROTO_ROOT/$PACKAGE_PROTO" >&2
  exit 1
fi

grpc_call() {
  local method="$1"
  grpcurl \
    -H "authorization: Bearer $AUTH_TOKEN" \
    -import-path "$PROTO_ROOT" \
    -proto "$PACKAGE_PROTO" \
    -d @ \
    "$PARTICIPANT_HOST" \
    "$method"
}

json_for_dar() {
  local dar="$1"
  local submission_id="$2"
  local action="$3"

  if [[ -n "${SYNCHRONIZER_ID:-}" ]]; then
    jq -n \
      --arg darFile "$(base64 < "$dar" | tr -d '\n')" \
      --arg submissionId "$submission_id" \
      --arg synchronizerId "$SYNCHRONIZER_ID" \
      --arg action "$action" \
      'if $action == "validate" then
         {
           darFile: $darFile,
           submissionId: $submissionId,
           synchronizerId: $synchronizerId
         }
       else
         {
           darFile: $darFile,
           submissionId: $submissionId,
           vettingChange: "VETTING_CHANGE_VET_ALL_PACKAGES",
           synchronizerId: $synchronizerId
         }
       end'
  else
    jq -n \
      --arg darFile "$(base64 < "$dar" | tr -d '\n')" \
      --arg submissionId "$submission_id" \
      --arg action "$action" \
      'if $action == "validate" then
         {
           darFile: $darFile,
           submissionId: $submissionId
         }
       else
         {
           darFile: $darFile,
           submissionId: $submissionId,
           vettingChange: "VETTING_CHANGE_VET_ALL_PACKAGES"
         }
       end'
  fi
}

validate_dar() {
  local dar="$1"
  local sid="validate-$(basename "$dar")-$(date +%s)"
  json_for_dar "$dar" "$sid" "validate" | grpc_call "com.daml.ledger.api.v2.admin.PackageManagementService/ValidateDarFile"
}

upload_dar() {
  local dar="$1"
  local sid="upload-$(basename "$dar")-$(date +%s)"
  json_for_dar "$dar" "$sid" "upload" | grpc_call "com.daml.ledger.api.v2.admin.PackageManagementService/UploadDarFile"
}

for dar in "${DARS[@]}"; do
  echo "==> $(basename "$dar")"
  if [[ "$VALIDATE_FIRST" == "1" ]]; then
    echo "    validating"
    validate_dar "$dar"
  fi
  echo "    uploading"
  upload_dar "$dar"
done

if [[ "$LIST_AFTER" == "1" ]]; then
  echo "==> listing known packages"
  printf '{}' | grpc_call "com.daml.ledger.api.v2.admin.PackageManagementService/ListKnownPackages"
fi
