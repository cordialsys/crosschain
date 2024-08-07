package address_test

import (
	"encoding/hex"
	"fmt"
	"testing"

	xc "github.com/cordialsys/crosschain"
	"github.com/cordialsys/crosschain/chain/cosmos/address"
	"github.com/cordialsys/crosschain/chain/cosmos/types/evmos/ethermint/crypto/ethsecp256k1"
	"github.com/cordialsys/crosschain/factory/signer"
	"github.com/cosmos/cosmos-sdk/crypto/keys/secp256k1"
	"github.com/stretchr/testify/require"
)

func TestNewAddressBuilder(t *testing.T) {
	builder, err := address.NewAddressBuilder(&xc.ChainConfig{})
	require.NoError(t, err)
	require.NotNil(t, builder)
}

func TestGetAddressFromPublicKey(t *testing.T) {
	builder, _ := address.NewAddressBuilder(&xc.ChainConfig{Chain: "LUNA", ChainPrefix: "terra"})
	bytes, _ := hex.DecodeString("02FCF724C97DFFAC2021EFA1818C2FEF3BCBB753CA22913A8DB5E79EC4A3DEE0D1")
	address, err := builder.GetAddressFromPublicKey(bytes)
	require.NoError(t, err)
	require.Equal(t, xc.Address("terra1dp3q305hgttt8n34rt8rg9xpanc42z4ye7upfg"), address)
}

func TestGetAddressFromPublicKeyEvmos(t *testing.T) {
	builder, _ := address.NewAddressBuilder(&xc.ChainConfig{Chain: "XPLA", ChainPrefix: "xpla", Driver: xc.DriverCosmosEvmos})
	bytes, _ := hex.DecodeString("02E8445082A72F29B75CA48748A914DF60622A609CACFCE8ED0E35804560741D29")
	address, err := builder.GetAddressFromPublicKey(bytes)
	require.NoError(t, err)
	require.Equal(t, xc.Address("xpla1r56x9533ntqtlsd99cth48fhyjf82gfstgvk9m"), address)
}

func TestGetAddressFromPublicKeyErr(t *testing.T) {
	builder, _ := address.NewAddressBuilder(&xc.ChainConfig{})

	require.Panics(t, func() {
		// cosmos-sdk panics with "length of pubkey is incorrect"
		_, _ = builder.GetAddressFromPublicKey([]byte{})
	})

	require.Panics(t, func() {
		// cosmos-sdk panics with "length of pubkey is incorrect"
		_, _ = builder.GetAddressFromPublicKey([]byte{1, 2, 3})
	})

	// AssetConfig.ChainPrefix is needed to bech32ify
	pubKeyBytes, _ := hex.DecodeString("02E8445082A72F29B75CA48748A914DF60622A609CACFCE8ED0E35804560741D29")
	derivedAddress, err := builder.GetAddressFromPublicKey(pubKeyBytes)
	require.Equal(t, xc.Address(""), derivedAddress)
	require.EqualError(t, err, "prefix cannot be empty")

	// cosmos-sdk doesn't check if pubkey is on the curve
	builder, _ = address.NewAddressBuilder(&xc.ChainConfig{Chain: "LUNA", ChainPrefix: "terra"})
	bytes, _ := hex.DecodeString("001122334455667788990011223344556677889900112233445566778899001122")
	derivedAddress, err = builder.GetAddressFromPublicKey(bytes)
	require.NoError(t, err)
	require.Equal(t, xc.Address("terra1hw58t56mzszlnnkjak83ul8ff437ylrz57xj4v"), derivedAddress)

	// ethermint doesn't check if pubkey is on the curve,
	// but it attempts to decompress the point to generate the address
	// therefore indirectly it catches the error
	builder, _ = address.NewAddressBuilder(&xc.ChainConfig{Chain: "XPLA", ChainPrefix: "xpla", Driver: xc.DriverCosmosEvmos})
	bytes, _ = hex.DecodeString("001122334455667788990011223344556677889900112233445566778899001122")
	derivedAddress, err = builder.GetAddressFromPublicKey(bytes)
	require.ErrorContains(t, err, "addresses cannot be empty")
	require.Equal(t, xc.Address(""), derivedAddress)
}

func TestGetAllPossibleAddressesFromPublicKey(t *testing.T) {
	builder, _ := address.NewAddressBuilder(&xc.ChainConfig{Chain: "LUNA", ChainPrefix: "terra"})
	bytes, _ := hex.DecodeString("02E8445082A72F29B75CA48748A914DF60622A609CACFCE8ED0E35804560741D29")
	addresses, err := builder.GetAllPossibleAddressesFromPublicKey(bytes)
	require.NoError(t, err)
	require.Equal(t, 1, len(addresses))
	require.Equal(t, xc.Address("terra1mzqd0kynsjzsnf3d37m5uvs53kkxssf0aasf27"), addresses[0].Address)
	require.Equal(t, xc.AddressTypeDefault, addresses[0].Type)
}

func TestKeyDerivation(t *testing.T) {

	type testcase struct {
		ChainCoinHDPath int
		ChainPrefix     string
		NativeAsset     xc.NativeAsset
		Mnemonic        string
		Address         string
		Driver          xc.Driver
	}
	for _, tc := range []testcase{
		{
			ChainCoinHDPath: 118,
			ChainPrefix:     "sei",
			NativeAsset:     "SEI",
			Mnemonic:        "protect scorpion scorpion answer joy question hollow short despair cube lumber stable toilet hat inflict fresh pigeon firm model foot excite broom dice gather",
			Address:         "sei1auf4yetx9z3lq5f93d8p8mm4j3lpt9s077m455",
			Driver:          xc.DriverCosmos,
		},
		{
			ChainCoinHDPath: 60,
			ChainPrefix:     "inj",
			NativeAsset:     "INJ",
			Mnemonic:        "captain dial clerk knee milk depart canyon stomach example smoke nominee vicious zero between enjoy outside exile original toddler casual broken roast episode car",
			Address:         "inj12szvunq39ky0lq20t9cy3tcs49n8k56v9q38fj",
			Driver:          xc.DriverCosmosEvmos,
		},
		{
			ChainCoinHDPath: 60,
			ChainPrefix:     "xpla",
			NativeAsset:     "XPLA",
			Mnemonic:        "script mercy language economy cat demand fruit page green license device air fatigue neither release mansion actor bitter latin toddler bright expand please salute",
			Address:         "xpla18tqp4j6ndm9fmly4t5mzgwh5zeg9rqrpm7zfnp",
			Driver:          xc.DriverCosmos,
		},
		{
			ChainCoinHDPath: 330,
			ChainPrefix:     "terra",
			NativeAsset:     "LUNA",
			Mnemonic:        "deer cotton advice flight female rotate health pond fortune guide soft brother broken sad gym pony rain lonely avoid chicken carpet trash tuna clean",
			Address:         "terra1evfrnqr9l5yxjp7ejektl2xmjdqlz08tuundzw",
			Driver:          xc.DriverCosmos,
		},
		{
			ChainCoinHDPath: 118,
			ChainPrefix:     "celestia",
			NativeAsset:     "TIA",
			Mnemonic:        "kid unique sadness clap embody grit sure couch crack muffin neutral rule cotton market apology cute brass laundry rural social exile peasant expand useless",
			Address:         "celestia1cl5k4awzka64ck974j4kshzezhmznpg66724xj",
			Driver:          xc.DriverCosmos,
		},
		{
			ChainCoinHDPath: 118,
			ChainPrefix:     "cosmos",
			NativeAsset:     "ATOM",
			Mnemonic:        "tide wage unit rack permit parent easy theme require focus honey connect intact furnace device tiger enter often cycle immense wire either better crush",
			Address:         "cosmos18jfym2e7gt7a5eclgawp4lwgh6n7ud77ak6vzt",
			Driver:          xc.DriverCosmos,
		},
		{
			ChainCoinHDPath: 1,
			ChainPrefix:     "tp",
			NativeAsset:     "HASH",
			Mnemonic:        "increase embark dice perfect october camera cousin matrix congress prosper fix what shiver staff undo airport master shadow swift level arch push industry gauge",
			Address:         "tp1x0wf90nl6rymz26d73l8hesk7neag82ka2zsv6",
			Driver:          xc.DriverCosmos,
		},
	} {

		asset := &xc.ChainConfig{
			ChainCoinHDPath: uint32(tc.ChainCoinHDPath),
			ChainPrefix:     tc.ChainPrefix,
			Chain:           tc.NativeAsset,
			Driver:          tc.Driver,
		}
		s, err := signer.New(tc.NativeAsset.Driver(), tc.Mnemonic, asset)
		require.NoError(t, err)
		pubkey, err := s.PublicKey()
		require.NoError(t, err)
		builder, err := address.NewAddressBuilder(asset)
		require.NoError(t, err)
		derivedAddress, err := builder.GetAddressFromPublicKey(pubkey)
		require.NoError(t, err)

		if tc.Address != string(derivedAddress) {
			// try to discover what the derivation path is
			for i := 0; i < 512; i++ {
				asset.ChainCoinHDPath = uint32(i)
				s, _ = signer.New(tc.NativeAsset.Driver(), tc.Mnemonic, asset)
				pubkey, _ = s.PublicKey()
				builder, _ = address.NewAddressBuilder(asset)
				otherAddress, _ := builder.GetAddressFromPublicKey(pubkey)
				if tc.Address == string(otherAddress) {
					fmt.Println("matching chain code: ", i, "produced expected address", otherAddress)
					break
				}
			}
		}

		require.EqualValues(t, tc.Address, derivedAddress)
	}

}

func TestIsEVMOS(t *testing.T) {
	is := address.IsEVMOS(&xc.ChainConfig{Chain: "ETH", Driver: xc.DriverEVM})
	require.False(t, is)

	is = address.IsEVMOS(&xc.ChainConfig{Chain: "ATOM", Driver: xc.DriverCosmos})
	require.False(t, is)

	is = address.IsEVMOS(&xc.ChainConfig{Chain: "LUNA", Driver: xc.DriverCosmos})
	require.False(t, is)

	is = address.IsEVMOS(&xc.ChainConfig{Chain: "XPLA", Driver: xc.DriverCosmos})
	require.False(t, is)

	is = address.IsEVMOS(&xc.ChainConfig{Chain: "XPLA", Driver: xc.DriverCosmosEvmos})
	require.True(t, is)
}

func TestGetPublicKey(t *testing.T) {
	pubKey := address.GetPublicKey(&xc.ChainConfig{Driver: xc.DriverCosmos}, []byte{})
	require.Exactly(t, &secp256k1.PubKey{Key: []byte{}}, pubKey)

	pubKey = address.GetPublicKey(&xc.ChainConfig{Driver: xc.DriverCosmosEvmos}, []byte{})
	require.Exactly(t, &ethsecp256k1.PubKey{Key: []byte{}}, pubKey)
}
