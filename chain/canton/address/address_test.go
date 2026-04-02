package address

import (
	"encoding/hex"
	"testing"

	xc "github.com/cordialsys/crosschain"
	"github.com/stretchr/testify/require"
)

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
	require.Equal(t, "122093aa96c5554371f0d1fd471ce282f3b590ab0758f35c124924c8e3715910bbe1", fingerprint)
}

func TestFingerprintFromPublicKey(t *testing.T) {
	t.Parallel()

	publicKeyBytes, err := hex.DecodeString("e5c86207770b9fb67d73eb4cb8cd6a5f6a5d6a63c66a5459bd77cca45fda6ede")
	require.NoError(t, err)

	fingerprint, err := FingerprintFromPublicKey(publicKeyBytes)
	require.NoError(t, err)
	require.Equal(t, "122079aa518eac66dcd662887155c5c7ee36d3b62e38ed0ded2ddc0c7050460bccc8", fingerprint)
}

func TestFingerprintFromPartyID(t *testing.T) {
	t.Parallel()

	addr := xc.Address("e5c86207770b9fb67d73eb4cb8cd6a5f6a5d6a63c66a5459bd77cca45fda6ede::122079aa518eac66dcd662887155c5c7ee36d3b62e38ed0ded2ddc0c7050460bccc8")

	fingerprint, err := FingerprintFromPartyID(addr)
	require.NoError(t, err)
	require.Equal(t, "122079aa518eac66dcd662887155c5c7ee36d3b62e38ed0ded2ddc0c7050460bccc8", fingerprint)
}

func TestParsePartyID(t *testing.T) {
	validFP := "1220" + hex.EncodeToString(make([]byte, 32)) // "1220" + 64 zeros

	tests := []struct {
		name         string
		addr         xc.Address
		expectErr    bool
		errContains  string
		expectedName string
		expectedFP   string
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
		expected := map[byte]string{
			0:   "1220ea618da83b6c6b2c4557ffa17d722045169f52b8f50f3b31fc867e266de7e53d",
			1:   "1220974cb80e78f2fea077628a02faa4c57d68a65036eea27fb3463088a1c8527a99",
			127: "122087fab42073577d1d066f0cc217b347aedf5a73ee5feaa75d5e96538dad977b91",
			255: "122044e19d94c296e8397d61e759ff1692e5dff8efbcd70d7a9b4033d8b4a259ccd0",
		}
		require.Equal(t, expected[seed], fp)
	}
}
