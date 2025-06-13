package tx_test

import (
	"testing"

	xc "github.com/cordialsys/crosschain"
	"github.com/cordialsys/crosschain/chain/filecoin/address"
	"github.com/cordialsys/crosschain/chain/filecoin/tx"
	"github.com/test-go/testify/require"
)

func TestTxHash(t *testing.T) {
	vectors := []struct {
		testName     string
		tx           tx.Tx
		expectedHash xc.TxHash
	}{
		{
			testName:     "EmptyMessage",
			tx:           tx.Tx{},
			expectedHash: "",
		},
		{
			// Cannot get hash from UnsignedMessage
			testName: "UnsignedMessage",
			tx: tx.Tx{
				Message: tx.Message{
					Version:    0,
					To:         "f13uhmulxtag3qfohj7h2nmtco7e7u3t3nxjdzi7q",
					From:       "f1urvqy4hx5idlki6b6f7ab6hzihjdfy47b5cc6dy",
					Nonce:      0,
					Value:      xc.NewAmountBlockchainFromStr("1"),
					GasLimit:   100000,
					GasFeeCap:  xc.NewAmountBlockchainFromStr("150000"),
					GasPremium: xc.NewAmountBlockchainFromStr("250000"),
					Method:     0,
					Params:     []byte{},
				},
			},
			expectedHash: "",
		},
		{
			testName: "SignedMessage",
			tx: tx.Tx{
				Message: tx.Message{
					Version:    0,
					To:         "f13uhmulxtag3qfohj7h2nmtco7e7u3t3nxjdzi7q",
					From:       "f1urvqy4hx5idlki6b6f7ab6hzihjdfy47b5cc6dy",
					Nonce:      0,
					Value:      xc.NewAmountBlockchainFromStr("1"),
					GasLimit:   100000,
					GasFeeCap:  xc.NewAmountBlockchainFromStr("150000"),
					GasPremium: xc.NewAmountBlockchainFromStr("250000"),
					Method:     0,
					Params:     []byte{},
				},
				Signature: tx.Signature{
					Type: address.ProtocolSecp256k1,
					Data: []byte("btOGs/+MfKwi02EQIdhvPdj8fw6xizsfCN6nMWCaR9YTSm8+ZjqYP5ggE8GzW0UrJd1zDgc1FNEwJYT6cxWElgE=\\"),
				},
			},
			expectedHash: "bafy2bzacedqhmyjl5ki3kjqwfg4piuzgdnryivexh5xjajqv2ijcxe5fhtpw4",
		},
	}
	for _, v := range vectors {
		t.Run(v.testName, func(t *testing.T) {
			require.Equal(t, v.expectedHash, v.tx.Hash())
		})
	}
}

func TestTxSighashes(t *testing.T) {
	vectors := []struct {
		testName           string
		tx                 tx.Tx
		expectedSigRequest []*xc.SignatureRequest
		err                string
	}{
		{
			testName:           "EmptyMessage",
			tx:                 tx.Tx{},
			expectedSigRequest: []*xc.SignatureRequest{},
			err:                "something??",
		},
		{
			testName: "MalformedAddress",
			tx: tx.Tx{
				Message: tx.Message{
					Version:    0,
					To:         "00000000000000000000000000000000000000000",
					From:       "f1urvqy4hx5idlki6b6f7ab6hzihjdfy47b5cc6dy",
					Nonce:      0,
					Value:      xc.NewAmountBlockchainFromStr("1"),
					GasLimit:   100000,
					GasFeeCap:  xc.NewAmountBlockchainFromStr("150000"),
					GasPremium: xc.NewAmountBlockchainFromStr("250000"),
					Method:     0,
					Params:     []byte{},
				},
				Signature: tx.Signature{
					Type: address.ProtocolSecp256k1,
					Data: []byte("btOGs/+MfKwi02EQIdhvPdj8fw6xizsfCN6nMWCaR9YTSm8+ZjqYP5ggE8GzW0UrJd1zDgc1FNEwJYT6cxWElgE=\\"),
				},
			},
			expectedSigRequest: []*xc.SignatureRequest{},
			err:                "invalid address",
		},
		{
			testName: "SignedMessage",
			tx: tx.Tx{
				Message: tx.Message{
					Version:    0,
					To:         "f13uhmulxtag3qfohj7h2nmtco7e7u3t3nxjdzi7q",
					From:       "f1urvqy4hx5idlki6b6f7ab6hzihjdfy47b5cc6dy",
					Nonce:      0,
					Value:      xc.NewAmountBlockchainFromStr("1"),
					GasLimit:   100000,
					GasFeeCap:  xc.NewAmountBlockchainFromStr("150000"),
					GasPremium: xc.NewAmountBlockchainFromStr("250000"),
					Method:     0,
					Params:     []byte{},
				},
				Signature: tx.Signature{
					Type: address.ProtocolSecp256k1,
					Data: []byte("btOGs/+MfKwi02EQIdhvPdj8fw6xizsfCN6nMWCaR9YTSm8+ZjqYP5ggE8GzW0UrJd1zDgc1FNEwJYT6cxWElgE=\\"),
				},
			},
			expectedSigRequest: []*xc.SignatureRequest{
				{
					Payload: []byte{
						248, 226, 214, 252, 199, 182, 133, 94, 80, 22,
						238, 8, 136, 168, 155, 138, 97, 14, 211, 19,
						51, 240, 77, 100, 27, 53, 29, 147, 142, 139,
						65, 46,
					},
				},
			},
			err: "",
		},
		{
			testName: "UnsignedMessage",
			tx: tx.Tx{
				Message: tx.Message{
					Version:    0,
					To:         "f13uhmulxtag3qfohj7h2nmtco7e7u3t3nxjdzi7q",
					From:       "f1urvqy4hx5idlki6b6f7ab6hzihjdfy47b5cc6dy",
					Nonce:      0,
					Value:      xc.NewAmountBlockchainFromStr("1"),
					GasLimit:   100000,
					GasFeeCap:  xc.NewAmountBlockchainFromStr("150000"),
					GasPremium: xc.NewAmountBlockchainFromStr("250000"),
					Method:     0,
					Params:     []byte{},
				},
			},
			expectedSigRequest: []*xc.SignatureRequest{
				{
					Payload: []byte{
						248, 226, 214, 252, 199, 182, 133, 94, 80, 22,
						238, 8, 136, 168, 155, 138, 97, 14, 211, 19,
						51, 240, 77, 100, 27, 53, 29, 147, 142, 139,
						65, 46,
					},
				},
			},
			err: "",
		},
	}
	for _, v := range vectors {
		t.Run(v.testName, func(t *testing.T) {
			actualSighash, err := v.tx.Sighashes()
			if len(v.err) > 0 {
				require.Error(t, err)
			} else {
				require.Equal(t, v.expectedSigRequest, actualSighash)
			}

		})
	}
}

func TestTxAddSignature(t *testing.T) {
	emptytx := tx.Tx{}
	err := emptytx.SetSignatures([]*xc.SignatureResponse{}...)
	require.EqualError(t, err, "only one signature is allowed")

	signedTx := tx.Tx{
		Signature: tx.Signature{
			Type: address.ProtocolSecp256k1,
			Data: []byte("signature"),
		},
	}
	err = signedTx.SetSignatures(&xc.SignatureResponse{Signature: []byte("asdf")})
	require.EqualError(t, err, "transaction already signed")
}
