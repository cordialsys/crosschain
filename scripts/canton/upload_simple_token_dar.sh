#!/usr/bin/env bash

set -euo pipefail

usage() {
  cat <<'EOF'
Upload the simple transfer-capable token implementation DAR to a Canton participant.

This targets:
  splice-token-test-simple-transfer

Required environment variables:
  PARTICIPANT_HOST   gRPC host:port of the participant
  AUTH_TOKEN         Bearer token with package upload rights

Optional environment variables:
  SYNCHRONIZER_ID    Target synchronizer/domain id for vetting
  PROTO_ROOT         Defaults to ~/Source/splice/canton/community/ledger-api/src/main/protobuf
  PACKAGE_PROTO      Defaults to com/daml/ledger/api/v2/admin/package_management_service.proto
  VALIDATE_FIRST     Defaults to 1. Set to 0 to skip ValidateDarFile calls.
  LIST_AFTER         Defaults to 1. Set to 0 to skip ListKnownPackages at the end.
  TOKEN_IMPL_DAR     Defaults to the built simple transfer DAR in the Splice repo.
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
TOKEN_IMPL_DAR="${TOKEN_IMPL_DAR:-$HOME/Source/splice/token-standard/examples/splice-token-test-simple-transfer/.daml/dist/splice-token-test-simple-transfer-0.0.1.dar}"

if [[ ! -f "$TOKEN_IMPL_DAR" ]]; then
  echo "DAR not found: $TOKEN_IMPL_DAR" >&2
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

if [[ "$VALIDATE_FIRST" == "1" ]]; then
  sid="validate-$(basename "$TOKEN_IMPL_DAR")-$(date +%s)"
  json_for_dar "$TOKEN_IMPL_DAR" "$sid" "validate" | grpc_call "com.daml.ledger.api.v2.admin.PackageManagementService/ValidateDarFile"
fi

sid="upload-$(basename "$TOKEN_IMPL_DAR")-$(date +%s)"
json_for_dar "$TOKEN_IMPL_DAR" "$sid" "upload" | grpc_call "com.daml.ledger.api.v2.admin.PackageManagementService/UploadDarFile"

if [[ "$LIST_AFTER" == "1" ]]; then
  printf '{}' | grpc_call "com.daml.ledger.api.v2.admin.PackageManagementService/ListKnownPackages"
fi
