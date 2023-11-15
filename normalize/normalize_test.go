package normalize_test

import (
	"testing"

	n "github.com/cordialsys/crosschain/normalize"
	"github.com/stretchr/testify/suite"
)

type NormalizeTestSuite struct {
	suite.Suite
}

func (s *NormalizeTestSuite) SetupTest() {
}

func TestCrosschain(t *testing.T) {
	suite.Run(t, new(NormalizeTestSuite))
}

func (s *NormalizeTestSuite) TestNormalize() {
	require := s.Require()
	address := ""

	address = n.Normalize("myaddress", "BTC")
	require.Equal("myaddress", address) // no normalization

	address = n.Normalize("bitcoincash:myaddress", "BCH")
	require.Equal("myaddress", address)

	address = n.Normalize("0x0ECE", "")
	require.Equal("0x0ece", address) // lowercase

	address = n.Normalize("0x0ECE", "ETH")
	require.Equal("0x0ece", address)
	address = n.Normalize("0x0ECE", "APTOS")
	require.Equal("0x0ece", address)
	address = n.Normalize("0x0ECE", "SUI")
	require.Equal("0x0ece", address)

	address = n.Normalize("0x0ECE", "XDC")
	require.Equal("xdc0ece", address)

	// no prefix
	address = n.Normalize("0x0ECE", "ETH", &n.NormalizeOptions{
		NoPrefix: true,
	})
	require.Equal("0ece", address)
	address = n.Normalize("0x0ECE", "APTOS", &n.NormalizeOptions{
		NoPrefix: true,
	})
	require.Equal("0ece", address)
	address = n.Normalize("0x0ECE", "SUI", &n.NormalizeOptions{
		NoPrefix: true,
	})
	require.Equal("0ece", address)

	// add the prefix back
	address = n.Normalize("0ECE", "ETH")
	require.Equal("0x0ece", address)
	address = n.Normalize("0ECE", "APTOS")
	require.Equal("0x0ece", address)

	// zero pad
	address = n.Normalize("0x0ECE", "ETH", &n.NormalizeOptions{
		ZeroPad: true,
	})
	require.Equal("0x0000000000000000000000000000000000000ece", address)

	address = n.Normalize("0xECE", "ETH", &n.NormalizeOptions{
		NoPrefix: true,
		ZeroPad:  true,
	})
	require.Equal("0000000000000000000000000000000000000ece", address)

	address = n.Normalize("0xECE", "APTOS", &n.NormalizeOptions{
		NoPrefix: true,
		ZeroPad:  true,
	})
	require.Equal("0000000000000000000000000000000000000000000000000000000000000ece", address)

	address = n.Normalize("0xECE", "SUI", &n.NormalizeOptions{
		NoPrefix: false,
		ZeroPad:  true,
	})
	require.Equal("0x0000000000000000000000000000000000000000000000000000000000000ece", address)

	// transaction hashes
	hash := n.Normalize("0x0ECE", "ETH", &n.NormalizeOptions{
		ZeroPad:         true,
		NoPrefix:        true,
		TransactionHash: true,
	})
	require.Equal("0000000000000000000000000000000000000000000000000000000000000ece", hash)

	hash = n.Normalize("0x0ECE", "APTOS", &n.NormalizeOptions{
		ZeroPad:         true,
		NoPrefix:        true,
		TransactionHash: true,
	})
	require.Equal("0000000000000000000000000000000000000000000000000000000000000ece", hash)

	hash = n.Normalize("Z1NLbnNcJkKvd8bg2WsmAoE741vMumbc27HHdQbzVyv", "SUI", &n.NormalizeOptions{
		NoPrefix:        false,
		ZeroPad:         true,
		TransactionHash: true,
	})
	require.Equal("Z1NLbnNcJkKvd8bg2WsmAoE741vMumbc27HHdQbzVyv", hash)

	// should return empty string still
	hash = n.Normalize("", "ETH", &n.NormalizeOptions{
		ZeroPad:         true,
		NoPrefix:        true,
		TransactionHash: true,
	})
	require.Equal("", hash)
}

func (s *NormalizeTestSuite) TestMoveAddressNormalize() {
	require := s.Require()
	// Test that only the hexadecimal string part of move addresses gets normalized
	// and that coin::Coin<> is removed
	naddr := n.NormalizeMoveAddress("0x11AAbbCCdd")
	require.Equal("0x11aabbccdd", naddr)

	naddr = n.NormalizeMoveAddress("0x11AAbbCCdd::coin::NAME")
	require.Equal("0x11aabbccdd::coin::NAME", naddr)

	naddr = n.NormalizeMoveAddress("coin::Coin<0x11AAbbCCdd::coin::NAME>")
	require.Equal("0x11aabbccdd::coin::NAME", naddr)

	naddr = n.NormalizeMoveAddress("coin::Coin<0x1::coin::NAME>")
	require.Equal("0x1::coin::NAME", naddr)
}

func (s *NormalizeTestSuite) TestNormalizeCosmos() {
	require := s.Require()
	address := n.Normalize("123456aABB", "LUNA", &n.NormalizeOptions{TransactionHash: true})
	require.Equal("123456aabb", address)
	// default should remove prefix
	address = n.Normalize("0x123456aABB", "LUNA", &n.NormalizeOptions{TransactionHash: true})
	require.Equal("123456aabb", address)

	address = n.Normalize("0x123456aABB", "LUNA", &n.NormalizeOptions{TransactionHash: true, ZeroPad: true})
	require.Equal("000000000000000000000000000000000000000000000000000000123456aabb", address)
}
