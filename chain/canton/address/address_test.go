package address

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"testing"

	xc "github.com/cordialsys/crosschain"
	"github.com/stretchr/testify/require"
)

// referenceFingerprint replicates Canton's fingerprint logic for test verification:
// SHA-256(bigEndianUint32(12) || rawPubKey), encoded as "1220" + hex(digest)
func referenceFingerprint(pubKey []byte) string {
	var purposeBytes [4]byte
	binary.BigEndian.PutUint32(purposeBytes[:], 12)
	h := sha256.New()
	h.Write(purposeBytes[:])
	h.Write(pubKey)
	return "1220" + hex.EncodeToString(h.Sum(nil))
}

func TestNewAddressBuilder(t *testing.T) {
	cfg := &xc.ChainBaseConfig{Chain: xc.CANTON, Driver: xc.DriverCanton}
	builder, err := NewAddressBuilder(cfg)
	require.NoError(t, err)
	require.NotNil(t, builder)
}

func TestGetAddressFromPublicKey(t *testing.T) {
	cfg := &xc.ChainBaseConfig{Chain: xc.CANTON, Driver: xc.DriverCanton}
	builder, err := NewAddressBuilder(cfg)
	require.NoError(t, err)

	pubKey := make([]byte, 32)
	for i := range pubKey {
		pubKey[i] = byte(i)
	}

	addr, err := builder.GetAddressFromPublicKey(pubKey)
	require.NoError(t, err)

	name, fingerprint, err := ParsePartyID(addr)
	require.NoError(t, err)

	// Name must be the hex-encoded public key
	require.Equal(t, hex.EncodeToString(pubKey), name)

	// Fingerprint must be "1220" + 64 hex chars (SHA-256 multihash)
	require.Equal(t, 68, len(fingerprint), "fingerprint must be 68 chars: 4 (1220) + 64 (SHA-256 hex)")
	require.Equal(t, "1220", fingerprint[:4])

	// Fingerprint value must match Canton's formula
	require.Equal(t, referenceFingerprint(pubKey), fingerprint)
}

func TestGetAddressFromPublicKeyInvalidLength(t *testing.T) {
	cfg := &xc.ChainBaseConfig{Chain: xc.CANTON, Driver: xc.DriverCanton}
	builder, err := NewAddressBuilder(cfg)
	require.NoError(t, err)

	for _, bad := range [][]byte{make([]byte, 16), make([]byte, 64), {}} {
		_, err := builder.GetAddressFromPublicKey(bad)
		require.Error(t, err)
		require.Contains(t, err.Error(), "invalid ed25519 public key length")
	}
}

func TestAddressDeterminism(t *testing.T) {
	cfg := &xc.ChainBaseConfig{Chain: xc.CANTON, Driver: xc.DriverCanton}
	builder, err := NewAddressBuilder(cfg)
	require.NoError(t, err)

	pubKey := make([]byte, 32)
	for i := range pubKey {
		pubKey[i] = byte(i * 7 % 256)
	}

	addr1, err := builder.GetAddressFromPublicKey(pubKey)
	require.NoError(t, err)
	addr2, err := builder.GetAddressFromPublicKey(pubKey)
	require.NoError(t, err)
	require.Equal(t, addr1, addr2)
}

func TestDifferentKeysProduceDifferentAddresses(t *testing.T) {
	cfg := &xc.ChainBaseConfig{Chain: xc.CANTON, Driver: xc.DriverCanton}
	builder, _ := NewAddressBuilder(cfg)

	key1 := make([]byte, 32)
	key2 := make([]byte, 32)
	key2[0] = 1

	addr1, _ := builder.GetAddressFromPublicKey(key1)
	addr2, _ := builder.GetAddressFromPublicKey(key2)
	require.NotEqual(t, addr1, addr2)
}

func TestParsePartyID(t *testing.T) {
	validFP := "1220" + hex.EncodeToString(make([]byte, 32)) // "1220" + 64 zeros

	tests := []struct {
		name          string
		addr          xc.Address
		expectErr     bool
		errContains   string
		expectedName  string
		expectedFP    string
	}{
		{
			name:         "valid - pubkey hex name",
			addr:         xc.Address("aabbccdd::" + validFP),
			expectedName: "aabbccdd",
			expectedFP:   validFP,
		},
		{
			name:         "valid - real-world style",
			addr:         xc.Address("example-validator-1::12201234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef"),
			expectedName: "example-validator-1",
			expectedFP:   "12201234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef",
		},
		{
			name:        "missing separator",
			addr:        xc.Address("nocolons1220aabbccdd"),
			expectErr:   true,
			errContains: "missing '::'",
		},
		{
			name:        "wrong prefix - old format",
			addr:        xc.Address("party::12aabbccdd"),
			expectErr:   true,
			errContains: "1220",
		},
		{
			name:        "wrong prefix - all zeros",
			addr:        xc.Address("party::0000" + hex.EncodeToString(make([]byte, 32))),
			expectErr:   true,
			errContains: "1220",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			name, fp, err := ParsePartyID(tt.addr)
			if tt.expectErr {
				require.Error(t, err)
				require.Contains(t, err.Error(), tt.errContains)
			} else {
				require.NoError(t, err)
				require.Equal(t, tt.expectedName, name)
				require.Equal(t, tt.expectedFP, fp)
			}
		})
	}
}

func TestFingerprintLength(t *testing.T) {
	// Any valid 32-byte key must produce a 68-char fingerprint
	cfg := &xc.ChainBaseConfig{Chain: xc.CANTON, Driver: xc.DriverCanton}
	builder, _ := NewAddressBuilder(cfg)

	for _, seed := range []byte{0, 1, 127, 255} {
		key := make([]byte, 32)
		for i := range key {
			key[i] = seed
		}
		addr, err := builder.GetAddressFromPublicKey(key)
		require.NoError(t, err)
		_, fp, err := ParsePartyID(addr)
		require.NoError(t, err)
		require.Equal(t, 68, len(fp), "fingerprint must always be 68 chars")
	}
}
