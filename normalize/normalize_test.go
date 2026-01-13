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
			// Bech32, should be no variation
			chain: xc.ADA,
			inp:   "addr1v9lwqlqytqnd0xgqd20fkfms4mfyvqmlzcxge7dwu6u8w8gpy60eg",
			out:   "addr1v9lwqlqytqnd0xgqd20fkfms4mfyvqmlzcxge7dwu6u8w8gpy60eg",
		},
		{
			chain: xc.APTOS,
			inp:   "0x0ECE",
			out:   "0x0000000000000000000000000000000000000000000000000000000000000ece",
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
			// should remove the bitcoincash: prefix
			chain: xc.BCH,
			inp:   "bitcoincash:qrwlh2l2ghsqefzre5xc5caw2nq52sr64v6kt98ymv",
			out:   "qrwlh2l2ghsqefzre5xc5caw2nq52sr64v6kt98ymv",
		},
		{
			// bech32, no variation
			chain: xc.BTC,
			inp:   "bc1pwy7vu875lwufqlcf6rr4fejjjdf0m9lhrpcrgt43kee6rnks262sm2x9ls",
			out:   "bc1pwy7vu875lwufqlcf6rr4fejjjdf0m9lhrpcrgt43kee6rnks262sm2x9ls",
		},
		{
			// bech32, no variation
			chain: xc.DOGE,
			inp:   "15BgrFpAb7kSBYCcCqZPbLDgNrqygmGumT",
			out:   "15BgrFpAb7kSBYCcCqZPbLDgNrqygmGumT",
		},
		{
			// base58, no variation
			chain: xc.DOT,
			inp:   "15EVqvzZ93gqafPTp7x4tCdSSx22mMFw8ypJj61wXH8DBZ3b",
			out:   "15EVqvzZ93gqafPTp7x4tCdSSx22mMFw8ypJj61wXH8DBZ3b",
		},
		{
			// should be no variation
			chain: xc.DUSK,
			inp:   "od2bVPYstYud4N1GFQo9bAP2YmCz3Nxw1iE2hEdooVSRHyLnxufnXRdxZMNZnG2NH3YaCxDCVC46SuRn1ALA6fs8XvyGTxLoWPqhLBM3GyHusbDAVTprjtGdJS8FU8bEqvx",
			out:   "od2bVPYstYud4N1GFQo9bAP2YmCz3Nxw1iE2hEdooVSRHyLnxufnXRdxZMNZnG2NH3YaCxDCVC46SuRn1ALA6fs8XvyGTxLoWPqhLBM3GyHusbDAVTprjtGdJS8FU8bEqvx",
		},
		{
			// should be no variation
			chain: xc.EOS,
			inp:   "EOS6FiFh5HgMKCQoU6NggepL7Xxd3JXUNPNvsLte6SdB8ss2h3Hfq",
			out:   "EOS6FiFh5HgMKCQoU6NggepL7Xxd3JXUNPNvsLte6SdB8ss2h3Hfq",
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
			// should be no variation, bech32
			chain: xc.FIL,
			inp:   "f12xj3kmjbpmc455c6cse2x43dda4j2xxlterpu5q",
			out:   "f12xj3kmjbpmc455c6cse2x43dda4j2xxlterpu5q",
		},
		{
			// lowercase hex, no 0x prefix
			chain: xc.ICP,
			inp:   "0x7650CC8DB0664F03919F8129BE519E17D5F82BEA7B3E423633B2D02E8D371E9B",
			out:   "7650cc8db0664f03919f8129be519e17d5f82bea7b3e423633b2d02e8d371e9b",
		},
		{
			chain: xc.ICP,
			inp:   "rmfui-qzccr-h74j3-hznfm-3w22n-kiyhd-mlggq-y37en-cjlmu-iyw7i-bae",
			out:   "rmfui-qzccr-h74j3-hznfm-3w22n-kiyhd-mlggq-y37en-cjlmu-iyw7i-bae",
		},
		{
			// required prefix and bech32, no variation
			chain: xc.KAS,
			inp:   "kaspa:qze7ykh5cf2uzcctcapf74lfwc4lz5xef25ctwlae0j3pg3wdvv35zhgnjs3e",
			out:   "kaspa:qze7ykh5cf2uzcctcapf74lfwc4lz5xef25ctwlae0j3pg3wdvv35zhgnjs3e",
		},
		{
			chain: xc.TON,
			inp:   "EQAjflEZ_6KgKMxPlcnKN1ZoUvHdTT6hVwTW95EGVQfeSha2",
			out:   "EQAjflEZ_6KgKMxPlcnKN1ZoUvHdTT6hVwTW95EGVQfeSha2",
		},

		{
			chain: xc.TRX,
			inp:   "TFrT3EMHdroQ6YSwLZtSLuWxFEbMbLibnE",
			out:   "TFrT3EMHdroQ6YSwLZtSLuWxFEbMbLibnE",
		},
		{
			// Base58, should be no variation
			chain: xc.SOL,
			inp:   "6MpyCTHuJBZv32RZXGs4Ao46y6YqzarqUkMkWb3WRq5y",
			out:   "6MpyCTHuJBZv32RZXGs4Ao46y6YqzarqUkMkWb3WRq5y",
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
			// Bech32, should be no variation
			chain: xc.XPLA,
			inp:   "xpla12qpz2g6acm42up4229mzqs25c8wx5t448n77um",
			out:   "xpla12qpz2g6acm42up4229mzqs25c8wx5t448n77um",
		},
		{
			// Base32, should be no variation
			chain: xc.XLM,
			inp:   "GBXGGNNOCK7YOH2OJUDGKCFKLFS47IQBCGYUZ5JHRVV3H7DD754NXH3S",
			out:   "GBXGGNNOCK7YOH2OJUDGKCFKLFS47IQBCGYUZ5JHRVV3H7DD754NXH3S",
		},
		{
			// Base58, should be no variation
			chain: xc.XRP,
			inp:   "rPUMVLA9XZwxvKPUGcdDPZRJvhReBCW3CQ",
			out:   "rPUMVLA9XZwxvKPUGcdDPZRJvhReBCW3CQ",
		},
		{
			chain: xc.HYPE,
			inp:   "0ECE", // don't add the prefix, it's not always required
			out:   "0x0ece",
		},
		{
			chain: xc.HYPE,
			inp:   "USDC:0x0ECE",
			out:   "USDC:0x0ece",
		},
		{
			chain: xc.HYPE,
			inp:   "USDC-0x0ECE",
			out:   "USDC-0x0ece",
		},
		{
			// Base58, should be no variation
			chain: xc.ZEC,
			inp:   "t1g4xVgMHVsxZWxS6D3SLXNXEAicivXKiAS",
			out:   "t1g4xVgMHVsxZWxS6D3SLXNXEAicivXKiAS",
		},
		{
			// normalize evm addresses
			chain: xc.HBAR,
			inp:   "0x0ECE",
			out:   "0x0ece",
		},
		{
			// leave hedera addressing intact
			chain: xc.HBAR,
			inp:   "0.0.111",
			out:   "0.0.111",
		},
		{
			// leave normalized intact
			chain: xc.HBAR,
			inp:   "0-0-111",
			out:   "0-0-111",
		},
		{
			// normalize implicit addresses
			chain: xc.NEAR,
			inp:   "0ECE",
			out:   "0ece",
		},
		{
			// leave near addresses intact
			chain: xc.NEAR,
			inp:   "crosschain.near",
			out:   "crosschain.near",
		},
		{
			// leave normalized intact
			chain: xc.NEAR,
			inp:   "crosschain-near",
			out:   "crosschain-near",
		},
	}

	// test that we have a test vector for each chain
	for _, chain := range xc.NativeAssetList {
		found := false
		for _, v := range vectors {
			if v.chain.Driver() == chain.Driver() {
				found = true
				break
			}
		}
		require.True(found, "No address normalization test vector for driver %s", chain.Driver())
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
