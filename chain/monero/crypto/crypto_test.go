package crypto

import (
	"encoding/hex"
	"testing"

	"github.com/stretchr/testify/require"
)

func hexDec(t *testing.T, s string) []byte {
	b, err := hex.DecodeString(s)
	require.NoError(t, err)
	return b
}

func TestHashToEC(t *testing.T) {
	// Test vectors from monero-project/tests/crypto/tests.txt
	// hash_to_ec <pubkey> <expected>
	vectors := []struct{ input, expected string }{
		{"da66e9ba613919dec28ef367a125bb310d6d83fb9052e71034164b6dc4f392d0", "52b3f38753b4e13b74624862e253072cf12f745d43fcfafbe8c217701a6e5875"},
		{"a7fbdeeccb597c2d5fdaf2ea2e10cbfcd26b5740903e7f6d46bcbf9a90384fc6", "f055ba2d0d9828ce2e203d9896bfda494d7830e7e3a27fa27d5eaa825a79a19c"},
		{"9ae78e5620f1c4e6b29d03da006869465b3b16dae87ab0a51f4e1b74bc8aa48b", "72d8720da66f797f55fbb7fa538af0b4a4f5930c8289c991472c37dc5ec16853"},
	}
	for _, v := range vectors {
		result := HashToEC(hexDec(t, v.input))
		require.Equal(t, v.expected, hex.EncodeToString(result), "hash_to_ec(%s)", v.input)
	}
}

func TestGenerateKeyDerivation(t *testing.T) {
	vectors := []struct{ pub, sec, expected string }{
		{"fdfd97d2ea9f1c25df773ff2c973d885653a3ee643157eb0ae2b6dd98f0b6984", "eb2bd1cf0c5e074f9dbf38ebbc99c316f54e21803048c687a3bb359f7a713b02", "4e0bd2c41325a1b89a9f7413d4d05e0a5a4936f241dccc3c7d0c539ffe00ef67"},
		{"1ebf8c3c296bb91708b09d9a8e0639ccfd72556976419c7dc7e6dfd7599218b9", "e49f363fd5c8fc1f8645983647ca33d7ec9db2d255d94cd538a3cc83153c5f04", "72903ec8f9919dfcec6efb5535490527b573b3d77f9890386d373c02bf368934"},
		{"3e3047a633b1f84250ae11b5c8e8825a3df4729f6cbe4713b887db62f268187d", "6df324e24178d91c640b75ab1c6905f8e6bb275bc2c2a5d9b9ecf446765a5a05", "9dcac9c9e87dd96a4115d84d587218d8bf165a0527153b1c306e562fe39a46ab"},
	}
	for _, v := range vectors {
		result, err := GenerateKeyDerivation(hexDec(t, v.pub), hexDec(t, v.sec))
		require.NoError(t, err)
		require.Equal(t, v.expected, hex.EncodeToString(result), "generate_key_derivation(%s, %s)", v.pub, v.sec)
	}
}

func TestDeriveKeysAndAddress(t *testing.T) {
	// Test our own key derivation produces a valid Monero address
	seed := hexDec(t, "c071fe9b1096538b047087a4ee3fdae204e4682eb2dfab78f3af752704b0f700")
	privSpend, privView, pubSpend, pubView, err := DeriveKeysFromSpend(seed)
	require.NoError(t, err)
	require.Len(t, privSpend, 32)
	require.Len(t, privView, 32)
	require.Len(t, pubSpend, 32)
	require.Len(t, pubView, 32)

	addr := GenerateAddress(pubSpend, pubView)
	// Monero mainnet addresses start with 4
	require.True(t, addr[0] == '4', "address should start with 4, got %s", addr[:5])
	require.Len(t, addr, 95, "standard address should be 95 chars")

	// Roundtrip test
	prefix, decodedSpend, decodedView, err := DecodeAddress(addr)
	require.NoError(t, err)
	require.Equal(t, MainnetAddressPrefix, prefix)
	require.Equal(t, hex.EncodeToString(pubSpend), hex.EncodeToString(decodedSpend))
	require.Equal(t, hex.EncodeToString(pubView), hex.EncodeToString(decodedView))
}

func TestScalarReduce(t *testing.T) {
	// Values < L should pass through unchanged
	small := hexDec(t, "0100000000000000000000000000000000000000000000000000000000000000")
	result := ScalarReduce(small)
	require.Equal(t, hex.EncodeToString(small), hex.EncodeToString(result))

	// Test that reduction works consistently
	hash := Keccak256([]byte("test"))
	r1 := ScalarReduce(hash)
	r2 := ScalarReduce(hash)
	require.Equal(t, hex.EncodeToString(r1), hex.EncodeToString(r2), "ScalarReduce should be deterministic")
}
