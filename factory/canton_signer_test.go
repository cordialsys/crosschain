package factory_test

import (
	"encoding/hex"
	"testing"

	xc "github.com/cordialsys/crosschain"
	"github.com/cordialsys/crosschain/factory"
	"github.com/stretchr/testify/require"
)

func TestCantonSignerCreation(t *testing.T) {
	// Create a testnet factory
	testnetFactory := factory.NewNotMainnetsFactory(&factory.FactoryOptions{})
	require.NotNil(t, testnetFactory)

	// Get Canton configuration
	config, found := testnetFactory.GetChain(xc.CANTON)
	require.True(t, found, "Canton should be configured")

	// Create a test private key (32 bytes for Ed25519)
	privateKeyHex := "0139472eff6886771a982f3083da5d421f24c29181e63888228dc81ca60d69e1"

	// Verify it's valid hex
	privateKeyBytes, err := hex.DecodeString(privateKeyHex)
	require.NoError(t, err)
	require.Len(t, privateKeyBytes, 32)

	// Try to create a signer - this is where the error would occur if signature type is not configured
	signer, err := testnetFactory.NewSigner(config.Base(), privateKeyHex)
	require.NoError(t, err, "Should be able to create Canton signer with Ed25519")
	require.NotNil(t, signer)

	// Get the public key
	publicKey, err := signer.PublicKey()
	require.NoError(t, err)
	require.NotNil(t, publicKey)
	require.Len(t, publicKey, 32, "Ed25519 public key should be 32 bytes")
}

func TestCantonAddressFromPrivateKey(t *testing.T) {
	// Create a testnet factory
	testnetFactory := factory.NewNotMainnetsFactory(&factory.FactoryOptions{})

	// Get Canton configuration
	config, found := testnetFactory.GetChain(xc.CANTON)
	require.True(t, found)

	// Create a test private key
	privateKeyHex := "0139472eff6886771a982f3083da5d421f24c29181e63888228dc81ca60d69e1"

	// Create a signer first, then get address from public key
	signer, err := testnetFactory.NewSigner(config.Base(), privateKeyHex)
	require.NoError(t, err)

	publicKey, err := signer.PublicKey()
	require.NoError(t, err)

	// Get address from public key
	address, err := testnetFactory.GetAddressFromPublicKey(config.Base(), publicKey)
	require.NoError(t, err, "Should be able to derive Canton address from public key")
	require.NotEmpty(t, address)

	// Verify the address format: <pubkey-hex>::1220<sha256-hex>
	addrStr := string(address)
	require.Contains(t, addrStr, "::", "Canton address should contain :: separator")
	require.Contains(t, addrStr, "::1220", "Canton address fingerprint should start with 1220 (SHA-256 multihash prefix)")
	// name (64 hex) + "::" (2) + "1220" (4) + 64 hex = 134 chars
	require.True(t, len(addrStr) > 70, "Canton address should be well over 70 characters")

	t.Logf("Generated Canton address: %s", addrStr)
}

func TestCantonSignatureAlgorithms(t *testing.T) {
	// Verify that Canton driver returns correct signature algorithms
	algorithms := xc.DriverCanton.SignatureAlgorithms()
	require.NotEmpty(t, algorithms, "Canton driver must return at least one signature algorithm")
	require.Len(t, algorithms, 1, "Canton should use exactly one signature algorithm")
	require.Equal(t, xc.Ed255, algorithms[0], "Canton must use Ed25519 (Ed255)")

	t.Logf("Canton signature algorithm: %s", algorithms[0])
}
