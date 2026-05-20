package canton

import (
	"strings"
	"testing"

	xc "github.com/cordialsys/crosschain"
	"github.com/stretchr/testify/require"
)

// validFP68 is a valid 68-char fingerprint: "1220" + 64 hex chars
const validFP68 = "12201234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef"

func TestValidateAddress(t *testing.T) {
	cfg := &xc.ChainBaseConfig{
		Chain:  xc.CANTON,
		Driver: xc.DriverCanton,
	}

	tests := []struct {
		name          string
		addr          xc.Address
		expectError   bool
		errorContains string
	}{
		{
			name:        "valid address",
			addr:        xc.Address("party::" + validFP68),
			expectError: false,
		},
		{
			name:        "valid address with complex name",
			addr:        xc.Address("my-party_123::" + validFP68),
			expectError: false,
		},
		{
			name:        "valid address with spaces",
			addr:        xc.Address("my party::" + validFP68),
			expectError: false,
		},
		{
			name:          "empty address",
			addr:          xc.Address(""),
			expectError:   true,
			errorContains: "empty address",
		},
		{
			name:          "missing separator",
			addr:          xc.Address("party1234567890abcdef"),
			expectError:   true,
			errorContains: "must contain exactly one '::' separator",
		},
		{
			name:          "multiple separators",
			addr:          xc.Address("party::name::" + validFP68),
			expectError:   true,
			errorContains: "must contain exactly one '::' separator",
		},
		{
			name:          "empty party name",
			addr:          xc.Address("::" + validFP68),
			expectError:   true,
			errorContains: "party name cannot be empty",
		},
		{
			name:          "party name too long",
			addr:          xc.Address(strings.Repeat("a", 186) + "::" + validFP68),
			expectError:   true,
			errorContains: "party name too long",
		},
		{
			name:          "invalid characters in party name",
			addr:          xc.Address("party@123::" + validFP68),
			expectError:   true,
			errorContains: "invalid characters",
		},
		{
			name:          "consecutive colons in party name",
			addr:          xc.Address("par::ty::" + validFP68),
			expectError:   true,
			errorContains: "must contain exactly one", // splits on :: so sees 3 parts
		},
		{
			name:          "fingerprint doesn't start with 1220",
			addr:          xc.Address("party::9934567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef12"),
			expectError:   true,
			errorContains: "must start with '1220'",
		},
		{
			name:          "fingerprint too short",
			addr:          xc.Address("party::1220abcdef"),
			expectError:   true,
			errorContains: "68 characters total",
		},
		{
			name:          "fingerprint too long",
			addr:          xc.Address("party::12201234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234"),
			expectError:   true,
			errorContains: "68 characters total",
		},
		{
			name:          "invalid hex in fingerprint",
			addr:          xc.Address("party::1220gggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggg"),
			expectError:   true,
			errorContains: "invalid hex characters",
		},
		{
			name:        "uppercase hex in fingerprint (should work)",
			addr:        xc.Address("party::12201234567890ABCDEF1234567890ABCDEF1234567890ABCDEF1234567890ABCDEF"),
			expectError: false,
		},
		{
			name:        "mixed case hex in fingerprint (should work)",
			addr:        xc.Address("party::12201234567890AbCdEf1234567890aBcDeF1234567890AbCdEf1234567890aBcDeF"),
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateAddress(cfg, tt.addr)

			if tt.expectError {
				require.Error(t, err)
				if tt.errorContains != "" {
					require.Contains(t, err.Error(), tt.errorContains)
				}
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestValidatePartyName(t *testing.T) {
	tests := []struct {
		name          string
		partyName     string
		expectError   bool
		errorContains string
	}{
		{
			name:        "valid simple name",
			partyName:   "party",
			expectError: false,
		},
		{
			name:        "valid with numbers",
			partyName:   "party123",
			expectError: false,
		},
		{
			name:        "valid with dash",
			partyName:   "my-party",
			expectError: false,
		},
		{
			name:        "valid with underscore",
			partyName:   "my_party",
			expectError: false,
		},
		{
			name:        "valid with space",
			partyName:   "my party",
			expectError: false,
		},
		{
			name:        "valid with colon",
			partyName:   "my:party",
			expectError: false,
		},
		{
			name:        "valid max length",
			partyName:   strings.Repeat("a", 185),
			expectError: false,
		},
		{
			name:          "empty name",
			partyName:     "",
			expectError:   true,
			errorContains: "cannot be empty",
		},
		{
			name:          "too long",
			partyName:     strings.Repeat("a", 186),
			expectError:   true,
			errorContains: "too long",
		},
		{
			name:          "invalid character @",
			partyName:     "party@123",
			expectError:   true,
			errorContains: "invalid characters",
		},
		{
			name:          "invalid character #",
			partyName:     "party#123",
			expectError:   true,
			errorContains: "invalid characters",
		},
		{
			name:          "consecutive colons",
			partyName:     "par::ty",
			expectError:   true,
			errorContains: "consecutive colons",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validatePartyName(tt.partyName)

			if tt.expectError {
				require.Error(t, err)
				if tt.errorContains != "" {
					require.Contains(t, err.Error(), tt.errorContains)
				}
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestValidateFingerprint(t *testing.T) {
	tests := []struct {
		name          string
		fingerprint   string
		expectError   bool
		errorContains string
	}{
		{
			name:        "valid fingerprint",
			fingerprint: "12201234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef",
			expectError: false,
		},
		{
			name:        "valid uppercase",
			fingerprint: "12201234567890ABCDEF1234567890ABCDEF1234567890ABCDEF1234567890ABCDEF",
			expectError: false,
		},
		{
			name:        "valid mixed case",
			fingerprint: "12201234567890AbCdEf1234567890aBcDeF1234567890AbCdEf1234567890aBcDeF",
			expectError: false,
		},
		{
			name:          "doesn't start with 1220",
			fingerprint:   "9934567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef12",
			expectError:   true,
			errorContains: "must start with '1220'",
		},
		{
			name:          "starts with 12 but not 1220",
			fingerprint:   "12341234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef",
			expectError:   true,
			errorContains: "must start with '1220'",
		},
		{
			name:          "too short",
			fingerprint:   "1220abcdef",
			expectError:   true,
			errorContains: "68 characters total",
		},
		{
			name:          "too long",
			fingerprint:   "12201234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234",
			expectError:   true,
			errorContains: "68 characters total",
		},
		{
			name:          "invalid hex characters - correct length",
			fingerprint:   "1220gggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggg",
			expectError:   true,
			errorContains: "invalid hex characters",
		},
		{
			name:          "contains spaces",
			fingerprint:   "12201234567890abcdef 234567890abcdef1234567890abcdef1234567890abcdef",
			expectError:   true,
			errorContains: "invalid hex characters",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateFingerprint(tt.fingerprint)

			if tt.expectError {
				require.Error(t, err)
				if tt.errorContains != "" {
					require.Contains(t, err.Error(), tt.errorContains)
				}
			} else {
				require.NoError(t, err)
			}
		})
	}
}
