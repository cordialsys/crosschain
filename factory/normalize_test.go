package factory

func (s *CrosschainTestSuite) TestNormalize() {
	require := s.Require()
	address := ""

	address = Normalize("myaddress", "BTC")
	require.Equal("myaddress", address) // no normalization

	address = Normalize("bitcoincash:myaddress", "BCH")
	require.Equal("myaddress", address)

	address = Normalize("0x0ECE", "")
	require.Equal("0x0ece", address) // lowercase

	address = Normalize("0x0ECE", "ETH")
	require.Equal("0x0ece", address)
	address = Normalize("0x0ECE", "APTOS")
	require.Equal("0x0ece", address)
	address = Normalize("0x0ECE", "SUI")
	require.Equal("0x0ece", address)

	address = Normalize("0x0ECE", "XDC")
	require.Equal("xdc0ece", address)

	// no prefix
	address = Normalize("0x0ECE", "ETH", &NormalizeOptions{
		NoPrefix: true,
	})
	require.Equal("0ece", address)
	address = Normalize("0x0ECE", "APTOS", &NormalizeOptions{
		NoPrefix: true,
	})
	require.Equal("0ece", address)
	address = Normalize("0x0ECE", "SUI", &NormalizeOptions{
		NoPrefix: true,
	})
	require.Equal("0ece", address)

	// add the prefix back
	address = Normalize("0ECE", "ETH")
	require.Equal("0x0ece", address)
	address = Normalize("0ECE", "APTOS")
	require.Equal("0x0ece", address)

	// zero pad
	address = Normalize("0x0ECE", "ETH", &NormalizeOptions{
		ZeroPad: true,
	})
	require.Equal("0x0000000000000000000000000000000000000ece", address)

	address = Normalize("0xECE", "ETH", &NormalizeOptions{
		NoPrefix: true,
		ZeroPad:  true,
	})
	require.Equal("0000000000000000000000000000000000000ece", address)

	address = Normalize("0xECE", "APTOS", &NormalizeOptions{
		NoPrefix: true,
		ZeroPad:  true,
	})
	require.Equal("0000000000000000000000000000000000000000000000000000000000000ece", address)

	address = Normalize("0xECE", "SUI", &NormalizeOptions{
		NoPrefix: false,
		ZeroPad:  true,
	})
	require.Equal("0x0000000000000000000000000000000000000000000000000000000000000ece", address)

	// transaction hashes
	hash := Normalize("0x0ECE", "ETH", &NormalizeOptions{
		ZeroPad:         true,
		NoPrefix:        true,
		TransactionHash: true,
	})
	require.Equal("0000000000000000000000000000000000000000000000000000000000000ece", hash)

	hash = Normalize("0x0ECE", "APTOS", &NormalizeOptions{
		ZeroPad:         true,
		NoPrefix:        true,
		TransactionHash: true,
	})
	require.Equal("0000000000000000000000000000000000000000000000000000000000000ece", hash)

	hash = Normalize("Z1NLbnNcJkKvd8bg2WsmAoE741vMumbc27HHdQbzVyv", "SUI", &NormalizeOptions{
		NoPrefix:        false,
		ZeroPad:         true,
		TransactionHash: true,
	})
	require.Equal("Z1NLbnNcJkKvd8bg2WsmAoE741vMumbc27HHdQbzVyv", hash)

	// should return empty string still
	hash = Normalize("", "ETH", &NormalizeOptions{
		ZeroPad:         true,
		NoPrefix:        true,
		TransactionHash: true,
	})
	require.Equal("", hash)
}

func (s *CrosschainTestSuite) TestMoveAddressNormalize() {
	require := s.Require()
	// Test that only the hexadecimal string part of move addresses gets normalized
	// and that coin::Coin<> is removed
	naddr := NormalizeMoveAddress("0x11AAbbCCdd")
	require.Equal("0x11aabbccdd", naddr)

	naddr = NormalizeMoveAddress("0x11AAbbCCdd::coin::NAME")
	require.Equal("0x11aabbccdd::coin::NAME", naddr)

	naddr = NormalizeMoveAddress("coin::Coin<0x11AAbbCCdd::coin::NAME>")
	require.Equal("0x11aabbccdd::coin::NAME", naddr)

	naddr = NormalizeMoveAddress("coin::Coin<0x1::coin::NAME>")
	require.Equal("0x1::coin::NAME", naddr)
}

func (s *CrosschainTestSuite) TestNormalizeCosmos() {
	require := s.Require()
	address := Normalize("123456aABB", "LUNA", &NormalizeOptions{TransactionHash: true})
	require.Equal("123456aabb", address)
	// default should remove prefix
	address = Normalize("0x123456aABB", "LUNA", &NormalizeOptions{TransactionHash: true})
	require.Equal("123456aabb", address)

	address = Normalize("0x123456aABB", "LUNA", &NormalizeOptions{TransactionHash: true, ZeroPad: true})
	require.Equal("000000000000000000000000000000000000000000000000000000123456aabb", address)
}
