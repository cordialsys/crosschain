#!/usr/bin/env bash

set -euo pipefail

if [[ -n "${_CANTON_TOKEN_STANDARD_LIB_SOURCED:-}" ]]; then
  return 0
fi
readonly _CANTON_TOKEN_STANDARD_LIB_SOURCED=1

require_cmd() {
  local cmd="$1"
  if ! command -v "$cmd" >/dev/null 2>&1; then
    echo "Missing required command: $cmd" >&2
    exit 1
  fi
}

require_cmd jq
require_cmd grpcurl
require_cmd curl

canton_resolve_auth_header() {
  if [[ -n "${AUTH_HEADER:-}" ]]; then
    printf '%s\n' "$AUTH_HEADER"
    return 0
  fi
  : "${AUTH_TOKEN:?AUTH_TOKEN is required unless AUTH_HEADER is provided}"
  printf 'Bearer %s\n' "$AUTH_TOKEN"
}

canton_resolve_scan_auth_header() {
  if [[ -n "${SCAN_AUTH_HEADER:-}" ]]; then
    printf '%s\n' "$SCAN_AUTH_HEADER"
    return 0
  fi
  : "${SCAN_AUTH_TOKEN:?SCAN_AUTH_TOKEN is required unless SCAN_AUTH_HEADER is provided}"
  printf 'Bearer %s\n' "$SCAN_AUTH_TOKEN"
}

canton_grpc_call() {
  local method="$1"
  local auth_header
  local tmp

  : "${PARTICIPANT_HOST:?PARTICIPANT_HOST is required}"
  auth_header="$(canton_resolve_auth_header)"
  tmp="$(mktemp)"
  if ! grpcurl \
    -H "authorization: $auth_header" \
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

canton_list_known_packages_json() {
  local response
  response="$(printf '{}' | canton_grpc_call "com.daml.ledger.api.v2.admin.PackageManagementService/ListKnownPackages")" || return 1
  if ! jq -e '(.package_details // .packageDetails) != null' >/dev/null <<<"$response"; then
    echo "Unexpected ListKnownPackages response:" >&2
    echo "$response" >&2
    return 1
  fi
  printf '%s\n' "$response"
}

canton_build_package_map_json() {
  canton_list_known_packages_json | jq '
    (.package_details // .packageDetails)
    | reduce .[] as $pkg ({}; . + {($pkg.name): ($pkg.package_id // $pkg.packageId)})
  '
}

canton_resolve_package_id_by_name() {
  local package_name="$1"
  canton_list_known_packages_json | jq -r --arg packageName "$package_name" '
    (.package_details // .packageDetails)
    | map(select(.name == $packageName))
    | sort_by((.known_since // .knownSince))
    | last
    | (.package_id // .packageId // empty)
  '
}

canton_get_ledger_end() {
  local response
  response="$(printf '{}' | canton_grpc_call "com.daml.ledger.api.v2.StateService/GetLedgerEnd")" || return 1
  if ! jq -e '(.offset // .ledger_end // .ledgerEnd) != null' >/dev/null <<<"$response"; then
    echo "Unexpected GetLedgerEnd response:" >&2
    echo "$response" >&2
    return 1
  fi
  jq -r '(.offset // .ledger_end // .ledgerEnd)' <<<"$response"
}

canton_get_token_holdings_json() {
  local party_id="$1"
  local holding_package_id="$2"
  local ledger_end="$3"

  jq -n \
    --arg party "$party_id" \
    --arg packageId "$holding_package_id" \
    --argjson ledgerEnd "$ledger_end" \
    '{
      activeAtOffset: $ledgerEnd,
      eventFormat: {
        filtersByParty: {
          ($party): {
            cumulative: [
              {
                interfaceFilter: {
                  interfaceId: {
                    packageId: $packageId,
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
      }
    }' \
    | canton_grpc_call "com.daml.ledger.api.v2.StateService/GetActiveContracts"
}

canton_get_transfer_instructions_json() {
  local party_id="$1"
  local transfer_package_id="$2"
  local ledger_end="$3"

  jq -n \
    --arg party "$party_id" \
    --arg packageId "$transfer_package_id" \
    --argjson ledgerEnd "$ledger_end" \
    '{
      activeAtOffset: $ledgerEnd,
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
      }
    }' \
    | canton_grpc_call "com.daml.ledger.api.v2.StateService/GetActiveContracts"
}

canton_scan_api_post() {
  local path="$1"
  local body_json="$2"
  local scan_auth_header
  local base_url
  local tmp
  local payload

  scan_auth_header="$(canton_resolve_scan_auth_header)"
  tmp="$(mktemp)"
  base_url="${SCAN_API_URL:-${REGISTRY_BASE_URL:-}}"

  if [[ -n "${SCAN_PROXY_URL:-}" ]]; then
    : "${base_url:?SCAN_API_URL is required when SCAN_PROXY_URL is used}"
    payload="$(
      jq -cn \
        --arg url "${base_url%/}${path}" \
        --arg body "$body_json" \
        '{
          method: "POST",
          url: $url,
          headers: { "Content-Type": "application/json" },
          body: $body
        }'
    )"
    if ! curl -fsSL \
      -H "Authorization: $scan_auth_header" \
      -H "Content-Type: application/json" \
      -d "$payload" \
      "${SCAN_PROXY_URL}" >"$tmp"; then
      cat "$tmp" >&2 || true
      rm -f "$tmp"
      return 1
    fi
  else
    : "${base_url:?SCAN_API_URL or REGISTRY_BASE_URL is required when SCAN_PROXY_URL is not set}"
    if ! curl -fsSL \
      -H "Authorization: $scan_auth_header" \
      -H "Content-Type: application/json" \
      -d "$body_json" \
      "${base_url%/}${path}" >"$tmp"; then
      cat "$tmp" >&2 || true
      rm -f "$tmp"
      return 1
    fi
  fi

  cat "$tmp"
  rm -f "$tmp"
}

canton_scan_api_get() {
  local path="$1"
  local scan_auth_header
  local base_url
  local tmp
  local payload

  scan_auth_header="$(canton_resolve_scan_auth_header)"
  tmp="$(mktemp)"
  base_url="${SCAN_API_URL:-${REGISTRY_BASE_URL:-}}"

  if [[ -n "${SCAN_PROXY_URL:-}" ]]; then
    : "${base_url:?SCAN_API_URL is required when SCAN_PROXY_URL is used}"
    payload="$(
      jq -cn \
        --arg url "${base_url%/}${path}" \
        '{
          method: "GET",
          url: $url,
          headers: { "Content-Type": "application/json" }
        }'
    )"
    if ! curl -fsSL \
      -H "Authorization: $scan_auth_header" \
      -H "Content-Type: application/json" \
      -d "$payload" \
      "${SCAN_PROXY_URL}" >"$tmp"; then
      cat "$tmp" >&2 || true
      rm -f "$tmp"
      return 1
    fi
  else
    : "${base_url:?SCAN_API_URL or REGISTRY_BASE_URL is required when SCAN_PROXY_URL is not set}"
    if ! curl -fsSL \
      -H "Authorization: $scan_auth_header" \
      "${base_url%/}${path}" >"$tmp"; then
      cat "$tmp" >&2 || true
      rm -f "$tmp"
      return 1
    fi
  fi

  cat "$tmp"
  rm -f "$tmp"
}

canton_party_fingerprint() {
  local party_id="$1"
  if [[ "$party_id" != *"::"* ]]; then
    echo "Invalid Canton party id: $party_id" >&2
    return 1
  fi
  printf '%s\n' "${party_id##*::}"
}

canton_ed25519_sign_prepared_hash_json() {
  local party_id="$1"
  local private_key_hex="$2"
  local hash_b64="$3"
  local tmp

  tmp="$(mktemp /tmp/canton-sign.XXXXXX.go)"
  cat >"$tmp" <<'EOF'
package main

import (
	"crypto/ed25519"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

type out struct {
	SignatureB64 string `json:"signature_b64"`
	PublicKeyHex string `json:"public_key_hex"`
}

func fail(msg string, args ...any) {
	fmt.Fprintf(os.Stderr, msg+"\n", args...)
	os.Exit(1)
}

func main() {
	if len(os.Args) != 4 {
		fail("usage: <party-id> <private-key-hex> <hash-b64>")
	}

	partyID := os.Args[1]
	privateKeyHex := strings.TrimSpace(os.Args[2])
	hashB64 := strings.TrimSpace(os.Args[3])

	parts := strings.SplitN(partyID, "::", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		fail("invalid Canton party id: %s", partyID)
	}
	partyName := strings.ToLower(parts[0])

	keyBytes, err := hex.DecodeString(privateKeyHex)
	if err != nil {
		fail("decode private key hex: %v", err)
	}

	var privateKey ed25519.PrivateKey
	switch len(keyBytes) {
	case ed25519.SeedSize:
		privateKey = ed25519.NewKeyFromSeed(keyBytes)
	case ed25519.PrivateKeySize:
		privateKey = ed25519.PrivateKey(keyBytes)
	default:
		fail("expected ed25519 key to be 32-byte seed or 64-byte private key, got %d bytes", len(keyBytes))
	}

	publicKey := privateKey.Public().(ed25519.PublicKey)
	publicKeyHex := strings.ToLower(hex.EncodeToString(publicKey))
	if publicKeyHex != partyName {
		fail("private key does not match PARTY_ID public key: expected %s, got %s", partyName, publicKeyHex)
	}

	hashBytes, err := base64.StdEncoding.DecodeString(hashB64)
	if err != nil {
		fail("decode prepared transaction hash: %v", err)
	}

	signature := ed25519.Sign(privateKey, hashBytes)
	if err := json.NewEncoder(os.Stdout).Encode(out{
		SignatureB64: base64.StdEncoding.EncodeToString(signature),
		PublicKeyHex: publicKeyHex,
	}); err != nil {
		fail("encode output: %v", err)
	}
}
EOF

  if ! go run "$tmp" "$party_id" "$private_key_hex" "$hash_b64"; then
    local status=$?
    rm -f "$tmp"
    return "$status"
  fi
  rm -f "$tmp"
}
