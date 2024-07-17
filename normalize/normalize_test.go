package normalize_test

import (
	"testing"

	xc "github.com/cordialsys/crosschain"
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

func (s *NormalizeTestSuite) TestNormalizeTransactionHash() {
	require := s.Require()

	type testcase struct {
		chain xc.NativeAsset
		inp   string
		out   string
	}
	vectors := []testcase{
		{
			chain: xc.BTC,
			inp:   "a0766c5c99739bcd6e41d3e42500400e6a1688c1c5cd0fff775f5e4c4137b071",
			out:   "a0766c5c99739bcd6e41d3e42500400e6a1688c1c5cd0fff775f5e4c4137b071",
		},
		{
			chain: xc.BTC,
			inp:   "0xAAABBBCCC",
			out:   "aaabbbccc",
		},
		{
			chain: xc.LUNA,
			inp:   "123456aABB",
			out:   "000000000000000000000000000000000000000000000000000000123456aabb",
		},
		{
			chain: xc.LUNA,
			inp:   "0x123456aABB",
			out:   "000000000000000000000000000000000000000000000000000000123456aabb",
		},
		{
			chain: xc.TON,
			inp:   "0x123456aABB",
			out:   "123456aabb",
		},
		{
			chain: xc.TON,
			inp:   "WkQx6xKpNhRBMMfHXykhcPknSc8IuNclmCEXFBily+8=",
			out:   "5a4431eb12a936144130c7c75f292170f92749cf08b8d7259821171418a5cbef",
		},
	}
	for _, v := range vectors {
		normalizedOut := n.TransactionHash(v.inp, v.chain)
		require.Equal(v.out, normalizedOut)

		normalizedOut2 := n.TransactionHash(v.inp, v.chain)
		require.Equal(normalizedOut, normalizedOut2, "Normalize should be idempotent")
	}
}

func (s *NormalizeTestSuite) TestNormalizeAddress() {
	require := s.Require()

	type testcase struct {
		chain xc.NativeAsset
		inp   string
		out   string
	}
	vectors := []testcase{
		{
			chain: xc.BTC,
			inp:   "myaddress",
			out:   "myaddress",
		},
		{
			chain: xc.BCH,
			inp:   "bitcoincash:myaddress",
			out:   "myaddress",
		},
		{
			// default to ETH if input has 0x
			chain: "",
			inp:   "0x0ECE",
			out:   "0x0ece",
		},
		{
			// do not default without 0x
			chain: "",
			inp:   "0ECE",
			out:   "0ECE",
		},
		{
			chain: xc.ETH,
			inp:   "0x0ECE",
			out:   "0x0ece",
		},
		{
			chain: xc.ETH,
			inp:   "0ECE", // add the prefix in
			out:   "0x0ece",
		},
		{
			chain: xc.APTOS,
			inp:   "0x0ECE",
			out:   "0x0ece",
		},
		{
			chain: xc.SUI,
			inp:   "0x0ECE",
			out:   "0x0ece",
		},
		{
			chain: xc.SUI,
			inp:   "coin::Coin<0x11AAbbCCdd::coin::NAME>",
			out:   "0x11aabbccdd::coin::NAME",
		},
		{
			chain: xc.APTOS,
			inp:   "coin::Coin<0x11AAbbCCdd::coin::NAME>",
			out:   "0x11aabbccdd::coin::NAME",
		},
		{
			chain: xc.APTOS,
			inp:   "other::Thing<0x11AAbbCCdd::coin::NAME>",
			out:   "other::Thing<0x11aabbccdd::coin::NAME>",
		},
		{
			chain: xc.XDC,
			inp:   "0x0ECE",
			out:   "xdc0ece",
		},
		{
			chain: xc.XDC,
			inp:   "xdc0ece",
			out:   "xdc0ece",
		},
		{
			chain: xc.SOL,
			inp:   "6MpyCTHuJBZv32RZXGs4Ao46y6YqzarqUkMkWb3WRq5y",
			out:   "6MpyCTHuJBZv32RZXGs4Ao46y6YqzarqUkMkWb3WRq5y",
		},
		{
			chain: xc.TRX,
			inp:   "TFrT3EMHdroQ6YSwLZtSLuWxFEbMbLibnE",
			out:   "TFrT3EMHdroQ6YSwLZtSLuWxFEbMbLibnE",
		},
		{
			chain: xc.TON,
			inp:   "EQAjflEZ_6KgKMxPlcnKN1ZoUvHdTT6hVwTW95EGVQfeSha2",
			out:   "EQAjflEZ_6KgKMxPlcnKN1ZoUvHdTT6hVwTW95EGVQfeSha2",
		},
	}
	for _, v := range vectors {
		normalizedOut := n.Normalize(v.inp, v.chain)
		require.Equal(v.out, normalizedOut)

		normalizedOut2 := n.Normalize(v.inp, v.chain)
		require.Equal(normalizedOut, normalizedOut2, "Normalize should be idempotent")

		addressId := normalizeResourceId(normalizedOut)
		addressIdNormalizedAgain := n.Normalize(addressId, v.chain)
		require.Equal(addressId, addressIdNormalizedAgain, "Normalize should not change an address after converted to ID compatible form")
	}

}

func (s *NormalizeTestSuite) TestMoveAddressNormalize() {
	require := s.Require()

	type testcase struct {
		inp string
		out string
	}
	vectors := []testcase{
		{
			inp: "0x11AAbbCCdd",
			out: "0x11aabbccdd",
		},
		{
			inp: "11AAbbCCdd",
			out: "0x11aabbccdd",
		},
		{
			inp: "0x11AAbbCCdd::coin::NAME",
			out: "0x11aabbccdd::coin::NAME",
		},
		{
			inp: "11AAbbCCdd::coin::NAME",
			out: "0x11aabbccdd::coin::NAME",
		},
		// only lowercase the hexidecimal part, even if using different separators
		{
			inp: "0x11AAbbCCdd--coin--NAME",
			out: "0x11aabbccdd--coin--NAME",
		},
		{
			inp: "0x11AAbbCCdd__coin__NAME",
			out: "0x11aabbccdd__coin__NAME",
		},
		{
			inp: "coin::Coin<0x11AAbbCCdd::coin::NAME>",
			out: "0x11aabbccdd::coin::NAME",
		},
		{
			inp: "coin::Coin<0x1::coin::NAME>",
			out: "0x1::coin::NAME",
		},
		{
			inp: "coin::Coin<1::coin::NAME>",
			out: "0x1::coin::NAME",
		},
	}
	for _, v := range vectors {
		out := n.NormalizeMoveAddress(v.inp)
		require.Equal(v.out, out)

		out2 := n.NormalizeMoveAddress(v.inp)
		require.Equal(out, out2, "NormalizeMoveAddress should be idempotent")

		normalizedOut := n.Normalize(v.inp, xc.SUI)
		require.Equal(v.out, normalizedOut, "Normalize should be the same as NormalizeMoveAddress for move chain")

		normalizedOut2 := n.Normalize(v.inp, xc.SUI)
		require.Equal(normalizedOut, normalizedOut2, "Normalize should be idempotent")
	}
}
