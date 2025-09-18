package tx_test

import (
	"encoding/hex"
	"testing"

	xc "github.com/cordialsys/crosschain"
	"github.com/cordialsys/crosschain/chain/hyperliquid/tx"
	"github.com/stretchr/testify/require"
)

func TestTxHash(t *testing.T) {
	tx := tx.Tx{
		Amount:           xc.NewAmountBlockchainFromUint64(10_000_000),
		Decimals:         8,
		Destination:      "0x21db009054831a7fd8914f544f749180630ce217",
		Token:            "HYPE:0x0d01dc56dcaaca66ad901c959b4011ec",
		Nonce:            1758098285252,
		HyperliquidChain: "Mainnet",
		Signature: tx.SignatureResult{
			R: "0x40fc49ad89ea1f664e08a897d24567c564270efda88a5e43e82247101eebd27c",
			S: "0x281b0fee061d4e4973cc9e215223dcf256d6bf5e2454c1f178f061ce0c4faea2",
			V: 28,
		},
	}
	require.Equal(t, xc.TxHash("0x2a1e9188244dfd737efcfb2e5dabfde8a4f8f2e3adeafbd46189ff32002ac4bc"), tx.Hash())
}

func TestTxSighashes(t *testing.T) {
	tx := tx.Tx{
		Amount:           xc.NewAmountBlockchainFromUint64(10_000_000),
		Decimals:         8,
		Destination:      "0x21db009054831a7fd8914f544f749180630ce217",
		Token:            "HYPE:0x0d01dc56dcaaca66ad901c959b4011ec",
		Nonce:            1758098505704,
		HyperliquidChain: "Mainnet",
	}

	sighashes, err := tx.Sighashes()
	require.NotNil(t, sighashes)
	require.NoError(t, err)

	expectedSighash, _ := hex.DecodeString("1d21f98b325ee360aa47700121db52af7c8306fe1d60eaf299031854e02da470")
	require.Equal(t, expectedSighash, sighashes[0].Payload)
}

func TestTxAddSignature(t *testing.T) {
	tx1 := tx.Tx{
		Amount:           xc.NewAmountBlockchainFromUint64(10_000_000),
		Decimals:         8,
		Destination:      "0x21db009054831a7fd8914f544f749180630ce217",
		Token:            "HYPE:0x0d01dc56dcaaca66ad901c959b4011ec",
		Nonce:            1758098790757,
		HyperliquidChain: "Mainnet",
	}

	signature, err := hex.DecodeString("72de6a68969db4d59796116554584b4c5f6b80ac39f49008a83819c726971356050ac27403b9c85ee640c91c838fe6d779f806053a8ebcd663d08bffe8ea2c5001")
	require.NoError(t, err)

	err = tx1.SetSignatures([]*xc.SignatureResponse{
		{
			Signature: signature,
		},
	}...)
	require.NoError(t, err)

	expectedSignature := tx.SignatureResult{
		R: "0x72de6a68969db4d59796116554584b4c5f6b80ac39f49008a83819c726971356",
		S: "0x50ac27403b9c85ee640c91c838fe6d779f806053a8ebcd663d08bffe8ea2c50",
		V: 28,
	}
	require.Equal(t, expectedSignature, tx1.Signature)
}
