package normalize_test

import (
	"fmt"
	"testing"

	xc "github.com/cordialsys/crosschain"
	n "github.com/cordialsys/crosschain/normalize"
	"github.com/stretchr/testify/require"
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
		{
			chain: xc.TRX,
			inp:   "8702B6CF6D722E1FF449373052A17D0534C810084973445141E3848B17AF0126",
			out:   "8702b6cf6d722e1ff449373052a17d0534c810084973445141e3848b17af0126",
		},
		{
			chain: xc.TRX,
			inp:   "0x722E1FF449373052A17D0534C810084973445141E3848B17AF0126",
			out:   "0000000000722e1ff449373052a17d0534c810084973445141e3848b17af0126",
		},
		{
			chain: xc.XLM,
			inp:   "3018226D96802A80A0CD93E95D4F012D6B546C536DBC96B9AD7FBE567ADC16D7",
			out:   "3018226d96802a80a0cd93e95d4f012d6b546c536dbc96b9ad7fbe567adc16d7",
		},
		{
			chain: xc.XLM,
			inp:   "0x18226D96802A80A0CD93E95D4F012D6B546C536DBC96B9AD7FBE567ADC16D7",
			out:   "0018226d96802a80a0cd93e95d4f012d6b546c536dbc96b9ad7fbe567adc16d7",
		},
		{
			chain: xc.XRP,
			inp:   "762D56FD2B28FB68CBCD3399B561931832C65AB940C772AD55536CAB9B3B7E9C",
			out:   "762d56fd2b28fb68cbcd3399b561931832c65ab940c772ad55536cab9b3b7e9c",
		},
		{
			chain: xc.XRP,
			inp:   "0x2D56FD2B28FB68CBCD3399B561931832C65AB940C772AD55536CAB9B3B7E9C",
			out:   "002d56fd2b28fb68cbcd3399b561931832c65ab940c772ad55536cab9b3b7e9c",
		},
	}
	for _, v := range vectors {
		normalizedOut := n.TransactionHash(v.inp, v.chain)
		require.Equal(v.out, normalizedOut)

		normalizedOut2 := n.TransactionHash(v.inp, v.chain)
		require.Equal(normalizedOut, normalizedOut2, "Normalize should be idempotent")
	}
}

func TestNormalizeAddress(t *testing.T) {
	require := require.New(t)

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
			out:   "0x0000000000000000000000000000000000000000000000000000000000000ece",
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
			chain: xc.APTOS,
			inp:   "0x89556578008574ed3fddda6bc2ea6bee475b042e237bbb2f447c263086edcc5",
			out:   "0x089556578008574ed3fddda6bc2ea6bee475b042e237bbb2f447c263086edcc5",
		},
		{
			chain: xc.APTOS,
			inp:   "0x2d91309b5b07a8be428ccd75d0443e81542ffcd059d0ab380cefc552229b1a",
			out:   "0x002d91309b5b07a8be428ccd75d0443e81542ffcd059d0ab380cefc552229b1a",
		},
		{
			chain: xc.APTOS,
			inp:   "0xb5b07a8be428ccd75d0443e81542ffcd059d0ab380cefc552229b1a",
			out:   "0x000000000b5b07a8be428ccd75d0443e81542ffcd059d0ab380cefc552229b1a",
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
		t.Run(fmt.Sprintf("%s-%s", v.chain, v.inp), func(t *testing.T) {
			normalizedOut := n.Normalize(v.inp, v.chain)
			require.Equal(v.out, normalizedOut)

			normalizedOut2 := n.Normalize(v.inp, v.chain)
			require.Equal(normalizedOut, normalizedOut2, "Normalize should be idempotent")

			addressId := normalizeId(normalizedOut)
			addressIdNormalizedAgain := n.Normalize(addressId, v.chain)
			require.Equal(addressId, addressIdNormalizedAgain, "Normalize should not change an address after converted to ID compatible form")
		})
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
