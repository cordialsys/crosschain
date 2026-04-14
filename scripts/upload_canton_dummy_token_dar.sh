#!/usr/bin/env bash

set -euo pipefail

usage() {
  cat <<'EOF'
Upload a concrete Canton token implementation DAR to a participant using grpcurl.

This script targets the smallest token-standard example implementation:
  splice-token-test-dummy-holding

Required environment variables:
  PARTICIPANT_HOST   gRPC host:port of the participant
  AUTH_TOKEN         Bearer token with package upload rights

Optional environment variables:
  SYNCHRONIZER_ID    Target synchronizer/domain id for vetting
  PROTO_ROOT         Defaults to ~/Source/splice/canton/community/ledger-api/src/main/protobuf
  PACKAGE_PROTO      Defaults to com/daml/ledger/api/v2/admin/package_management_service.proto
  VALIDATE_FIRST     Defaults to 1. Set to 0 to skip ValidateDarFile calls.
  LIST_AFTER         Defaults to 1. Set to 0 to skip ListKnownPackages at the end.
  TOKEN_IMPL_DAR     Defaults to the built dummy holding DAR in the Splice repo.

Before using this script:
  1. Upload the token-standard interface DARs first.
  2. Build the implementation DAR if it is missing.
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
TOKEN_IMPL_DAR="${TOKEN_IMPL_DAR:-$HOME/Source/splice/token-standard/examples/splice-token-test-dummy-holding/.daml/dist/splice-token-test-dummy-holding-0.0.2.dar}"

if [[ ! -f "$TOKEN_IMPL_DAR" ]]; then
  echo "DAR not found: $TOKEN_IMPL_DAR" >&2
  echo "Build it first, for example:" >&2
  echo '  cd ~/Source/splice/token-standard/examples/splice-token-test-dummy-holding' >&2
  echo '  ~/.cache/daml-build/3.3.0-snapshot.20250502.13767.0.v2fc6c7e2/damlc/damlc build' >&2
  exit 1
fi

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

echo "==> $(basename "$TOKEN_IMPL_DAR")"
if [[ "$VALIDATE_FIRST" == "1" ]]; then
  echo "    validating"
  validate_dar "$TOKEN_IMPL_DAR"
fi
echo "    uploading"
upload_dar "$TOKEN_IMPL_DAR"

if [[ "$LIST_AFTER" == "1" ]]; then
  echo "==> listing known packages"
  printf '{}' | grpc_call "com.daml.ledger.api.v2.admin.PackageManagementService/ListKnownPackages"
fi
