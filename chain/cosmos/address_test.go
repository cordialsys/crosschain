package cosmos

import (
	"encoding/hex"

	xc "github.com/jumpcrypto/crosschain"
)

func (s *CrosschainTestSuite) TestNewAddressBuilder() {
	require := s.Require()
	builder, err := NewAddressBuilder(&xc.AssetConfig{})
	require.Nil(err)
	require.NotNil(builder)
}

func (s *CrosschainTestSuite) TestGetAddressFromPublicKey() {
	require := s.Require()
	builder, _ := NewAddressBuilder(&xc.AssetConfig{NativeAsset: "LUNA", ChainPrefix: "terra"})
	bytes, _ := hex.DecodeString("02FCF724C97DFFAC2021EFA1818C2FEF3BCBB753CA22913A8DB5E79EC4A3DEE0D1")
	address, err := builder.GetAddressFromPublicKey(bytes)
	require.Nil(err)
	require.Equal(xc.Address("terra1dp3q305hgttt8n34rt8rg9xpanc42z4ye7upfg"), address)
}

func (s *CrosschainTestSuite) TestGetAddressFromPublicKeyEvmos() {
	require := s.Require()
	builder, _ := NewAddressBuilder(&xc.AssetConfig{NativeAsset: "XPLA", ChainPrefix: "xpla", Driver: string(xc.DriverCosmosEvmos)})
	bytes, _ := hex.DecodeString("02E8445082A72F29B75CA48748A914DF60622A609CACFCE8ED0E35804560741D29")
	address, err := builder.GetAddressFromPublicKey(bytes)
	require.Nil(err)
	require.Equal(xc.Address("xpla1r56x9533ntqtlsd99cth48fhyjf82gfstgvk9m"), address)
}

func (s *CrosschainTestSuite) TestGetAddressFromPublicKeyErr() {
	require := s.Require()
	builder, _ := NewAddressBuilder(&xc.AssetConfig{})

	require.Panics(func() {
		// cosmos-sdk panics with "length of pubkey is incorrect"
		_, _ = builder.GetAddressFromPublicKey([]byte{})
	})

	require.Panics(func() {
		// cosmos-sdk panics with "length of pubkey is incorrect"
		_, _ = builder.GetAddressFromPublicKey([]byte{1, 2, 3})
	})

	// AssetConfig.ChainPrefix is needed to bech32ify
	pubKeyBytes, _ := hex.DecodeString("02E8445082A72F29B75CA48748A914DF60622A609CACFCE8ED0E35804560741D29")
	address, err := builder.GetAddressFromPublicKey(pubKeyBytes)
	require.Equal(xc.Address(""), address)
	require.EqualError(err, "prefix cannot be empty")

	// cosmos-sdk doesn't check if pubkey is on the curve
	builder, _ = NewAddressBuilder(&xc.AssetConfig{NativeAsset: "LUNA", ChainPrefix: "terra"})
	bytes, _ := hex.DecodeString("001122334455667788990011223344556677889900112233445566778899001122")
	address, err = builder.GetAddressFromPublicKey(bytes)
	require.Nil(err)
	require.Equal(xc.Address("terra1hw58t56mzszlnnkjak83ul8ff437ylrz57xj4v"), address)

	// ethermint doesn't check if pubkey is on the curve,
	// but it attempts to decompress the point to generate the address
	// therefore indirectly it catches the error
	builder, _ = NewAddressBuilder(&xc.AssetConfig{NativeAsset: "XPLA", ChainPrefix: "xpla", Driver: string(xc.DriverCosmosEvmos)})
	bytes, _ = hex.DecodeString("001122334455667788990011223344556677889900112233445566778899001122")
	address, err = builder.GetAddressFromPublicKey(bytes)
	require.ErrorContains(err, "addresses cannot be empty")
	require.Equal(xc.Address(""), address)
}

func (s *CrosschainTestSuite) TestGetAllPossibleAddressesFromPublicKey() {
	require := s.Require()
	builder, _ := NewAddressBuilder(&xc.AssetConfig{NativeAsset: "LUNA", ChainPrefix: "terra"})
	bytes, _ := hex.DecodeString("02E8445082A72F29B75CA48748A914DF60622A609CACFCE8ED0E35804560741D29")
	addresses, err := builder.GetAllPossibleAddressesFromPublicKey(bytes)
	require.Nil(err)
	require.Equal(1, len(addresses))
	require.Equal(xc.Address("terra1mzqd0kynsjzsnf3d37m5uvs53kkxssf0aasf27"), addresses[0].Address)
	require.Equal(xc.AddressTypeDefault, addresses[0].Type)
}

func (s *CrosschainTestSuite) TestKeyDerivation() {
	require := s.Require()

	type testcase struct {
		ChainCoinHDPath int
		ChainPrefix     string
		NativeAsset     string
		Mnemonic        string
		Address         string
	}
	for _, tc := range []testcase{
		{
			ChainCoinHDPath: 118,
			ChainPrefix:     "sei",
			NativeAsset:     "SEI",
			Mnemonic:        "protect scorpion scorpion answer joy question hollow short despair cube lumber stable toilet hat inflict fresh pigeon firm model foot excite broom dice gather",
			Address:         "sei1auf4yetx9z3lq5f93d8p8mm4j3lpt9s077m455",
		},
		{
			ChainCoinHDPath: 60,
			ChainPrefix:     "inj",
			NativeAsset:     "INJ",
			Mnemonic:        "captain dial clerk knee milk depart canyon stomach example smoke nominee vicious zero between enjoy outside exile original toddler casual broken roast episode car",
			Address:         "inj12szvunq39ky0lq20t9cy3tcs49n8k56v9q38fj",
		},
		{
			ChainCoinHDPath: 60,
			ChainPrefix:     "xpla",
			NativeAsset:     "XPLA",
			Mnemonic:        "script mercy language economy cat demand fruit page green license device air fatigue neither release mansion actor bitter latin toddler bright expand please salute",
			Address:         "xpla18tqp4j6ndm9fmly4t5mzgwh5zeg9rqrpm7zfnp",
		},
		{
			ChainCoinHDPath: 330,
			ChainPrefix:     "terra",
			NativeAsset:     "LUNA",
			Mnemonic:        "deer cotton advice flight female rotate health pond fortune guide soft brother broken sad gym pony rain lonely avoid chicken carpet trash tuna clean",
			Address:         "terra1evfrnqr9l5yxjp7ejektl2xmjdqlz08tuundzw",
		},
		{
			ChainCoinHDPath: 118,
			ChainPrefix:     "celestia",
			NativeAsset:     "TIA",
			Mnemonic:        "kid unique sadness clap embody grit sure couch crack muffin neutral rule cotton market apology cute brass laundry rural social exile peasant expand useless",
			Address:         "celestia1cl5k4awzka64ck974j4kshzezhmznpg66724xj",
		},
		{
			ChainCoinHDPath: 118,
			ChainPrefix:     "cosmos",
			NativeAsset:     "ATOM",
			Mnemonic:        "tide wage unit rack permit parent easy theme require focus honey connect intact furnace device tiger enter often cycle immense wire either better crush",
			Address:         "cosmos18jfym2e7gt7a5eclgawp4lwgh6n7ud77ak6vzt",
		},
	} {

		asset := &xc.NativeAssetConfig{
			ChainCoinHDPath: uint32(tc.ChainCoinHDPath),
			ChainPrefix:     tc.ChainPrefix,
			NativeAsset:     xc.NativeAsset(tc.NativeAsset),
		}
		signer, err := NewSigner(asset)
		require.NoError(err)
		privkey, err := signer.ImportPrivateKey(tc.Mnemonic)
		require.NoError(err)
		pubkey, err := signer.PublicKey(privkey)
		require.NoError(err)
		builder, err := NewAddressBuilder(asset)
		require.NoError(err)
		address, err := builder.GetAddressFromPublicKey(pubkey)
		require.NoError(err)

		require.EqualValues(tc.Address, address)
	}

}
