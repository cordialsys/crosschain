package address_test

import (
	"encoding/hex"
	"testing"

	xc "github.com/cordialsys/crosschain"
	xcaddress "github.com/cordialsys/crosschain/address"
	"github.com/cordialsys/crosschain/chain/internet_computer/address"
	"github.com/stretchr/testify/require"
)

func TestNewAddressBuilder(t *testing.T) {
	builder, err := address.NewAddressBuilder(xc.NewChainConfig("XYZ").Base())
	require.NotNil(t, builder)
	require.NoError(t, err)
}

func TestGetAddressFromPublicKey(t *testing.T) {
	pk := "bd08143ec55c47d3be603f8cf395025f8473d0e4d09a72eb83631fc1d745fb31"
	pkBytes, err := hex.DecodeString(pk)
	require.NoError(t, err)

	builder, _ := address.NewAddressBuilder(xc.NewChainConfig("XYZ").Base())
	address, err := builder.GetAddressFromPublicKey(pkBytes)
	require.Equal(t, xc.Address("6c5066261553064a8d4fa8f30fa9d587d9887bce69601cdb5b6cac8780fc8899"), address)
	require.NoError(t, err)
}

func TestGetTokenAddressFromPublicKey(t *testing.T) {
	pk := "bd08143ec55c47d3be603f8cf395025f8473d0e4d09a72eb83631fc1d745fb31"
	pkBytes, err := hex.DecodeString(pk)
	require.NoError(t, err)

	addressArgs := []xcaddress.AddressOption{}
	addressArgs = append(addressArgs, xcaddress.OptionContract(xc.ContractAddress("any-works")))
	builder, _ := address.NewAddressBuilder(xc.NewChainConfig("ICP").Base(), addressArgs...)

	address, err := builder.GetAddressFromPublicKey(pkBytes)
	require.Equal(t, xc.Address("mglk4-25zez-he5uh-lsy2a-bontn-pfarj-offxd-5teb2-icnpp-scmni-zae"), address)
	require.NoError(t, err)
}
