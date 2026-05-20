package canton

import (
	"testing"

	xc "github.com/cordialsys/crosschain"
	"github.com/stretchr/testify/require"
)

func TestCantonSignatureAlgorithm(t *testing.T) {
	driver := xc.DriverCanton

	// Verify Canton returns at least one signature algorithm
	algorithms := driver.SignatureAlgorithms()
	require.NotEmpty(t, algorithms, "Canton driver should return at least one signature algorithm")
	require.Len(t, algorithms, 1, "Canton should have exactly one signature algorithm")

	// Verify it's Ed25519
	require.Equal(t, xc.Ed255, algorithms[0], "Canton should use Ed25519 signatures")
}

func TestCantonPublicKeyFormat(t *testing.T) {
	driver := xc.DriverCanton

	// Verify Canton uses Raw public key format
	format := driver.PublicKeyFormat()
	require.Equal(t, xc.Raw, format, "Canton should use Raw public key format")
}
