package tx_test

import (
	"testing"

	xc "github.com/cordialsys/crosschain"
	tx "github.com/cordialsys/crosschain/chain/xlm/tx"
	"github.com/stellar/go/xdr"
	"github.com/test-go/testify/require"
)

const TESTNET_NETWORK_PASSPHRASE = "Test SDF Network ; September 2015"

func TestTxHash(t *testing.T) {
	from := xc.Address("GDLO3EPTGZIC75YG3F3STV5LKUQ6EMGDSNJ4U6JXFUVR7QRZ5KTSYRJF")
	var source xdr.MuxedAccount
	err := source.SetAddress(string(from))
	require.NoError(t, err)

	to := xc.Address("GCITKPHEIYPB743IM4DYB23IOZIRBAQ76J6QNKPPXVI2N575JZ3Z65DI")
	var destination xdr.MuxedAccount
	err = destination.SetAddress(string(to))
	require.NoError(t, err)

	require.NoError(t, err)
	preconditions := tx.Preconditions{
		TimeBounds: tx.NewInfiniteTimeout(),
	}

	vectors := []struct {
		tx           tx.Tx
		expectedHash xc.TxHash
	}{
		{
			tx:           tx.Tx{},
			expectedHash: "",
		},
		{
			tx: tx.Tx{
				TxEnvelope: &xdr.TransactionEnvelope{
					Type: xdr.EnvelopeTypeEnvelopeTypeTx,
					V1: &xdr.TransactionV1Envelope{
						Tx: xdr.Transaction{
							SourceAccount: source,
							Fee:           xdr.Uint32(100),
							SeqNum:        338194314821647,
							Cond:          preconditions.BuildXDR(),
							Operations: []xdr.Operation{
								{
									SourceAccount: &source,
									Body: xdr.OperationBody{
										Type: xdr.OperationTypePayment,
										PaymentOp: &xdr.PaymentOp{
											Asset:       xdr.Asset{Type: xdr.AssetTypeAssetTypeNative},
											Destination: destination,
											Amount:      xdr.Int64(10000000),
										},
									},
								},
							},
						},
					},
				},
				NetworkPassphrase: TESTNET_NETWORK_PASSPHRASE,
			},
			expectedHash: "1ea250b425a38ed08bab46a141b467e19a2789548c36abc0f4ae3e363fd8a1f3",
		},
	}

	for _, vector := range vectors {
		actualHash := vector.tx.Hash()
		require.Equal(t, vector.expectedHash, actualHash)
	}
}

func TestTxSighashes(t *testing.T) {
	from := xc.Address("GDLO3EPTGZIC75YG3F3STV5LKUQ6EMGDSNJ4U6JXFUVR7QRZ5KTSYRJF")
	var source xdr.MuxedAccount
	err := source.SetAddress(string(from))
	require.NoError(t, err)

	to := xc.Address("GCITKPHEIYPB743IM4DYB23IOZIRBAQ76J6QNKPPXVI2N575JZ3Z65DI")
	var destination xdr.MuxedAccount
	err = destination.SetAddress(string(to))
	require.NoError(t, err)

	require.NoError(t, err)
	preconditions := tx.Preconditions{
		TimeBounds: tx.NewInfiniteTimeout(),
	}

	vectors := []struct {
		tx              tx.Tx
		expectedSigHash []xc.TxDataToSign
		err             string
	}{
		{
			tx:              tx.Tx{},
			expectedSigHash: nil,
			err:             "transaction envelope is missing",
		},
		{
			tx: tx.Tx{
				TxEnvelope: &xdr.TransactionEnvelope{
					Type: xdr.EnvelopeTypeEnvelopeTypeTx,
					V1: &xdr.TransactionV1Envelope{
						Tx: xdr.Transaction{
							SourceAccount: source,
							Fee:           xdr.Uint32(100),
							SeqNum:        338194314821647,
							Cond:          preconditions.BuildXDR(),
							Operations: []xdr.Operation{
								{
									SourceAccount: &source,
									Body: xdr.OperationBody{
										Type: xdr.OperationTypePayment,
										PaymentOp: &xdr.PaymentOp{
											Asset:       xdr.Asset{Type: xdr.AssetTypeAssetTypeNative},
											Destination: destination,
											Amount:      xdr.Int64(10000000),
										},
									},
								},
							},
						},
					},
				},
				NetworkPassphrase: TESTNET_NETWORK_PASSPHRASE,
			},
			expectedSigHash: []xc.TxDataToSign{
				{
					0x1e, 0xa2, 0x50, 0xb4, 0x25, 0xa3,
					0x8e, 0xd0, 0x8b, 0xab, 0x46, 0xa1,
					0x41, 0xb4, 0x67, 0xe1, 0x9a, 0x27,
					0x89, 0x54, 0x8c, 0x36, 0xab, 0xc0,
					0xf4, 0xae, 0x3e, 0x36, 0x3f, 0xd8,
					0xa1, 0xf3,
				},
			},
		},
	}

	for _, vector := range vectors {
		sigHash, err := vector.tx.Sighashes()
		require.Equal(t, vector.expectedSigHash, sigHash)
		if err != nil {
			require.Error(t, err)
		} else {
			require.Nil(t, err)
		}
	}
}

func TestTxAddSignature(t *testing.T) {
	tx0 := tx.Tx{}
	err := tx0.AddSignatures([]xc.TxSignature{}...)
	require.EqualError(t, err, "missing transaction envelope")

	tx1 := tx.Tx{
		TxEnvelope: &xdr.TransactionEnvelope{},
		Signatures: []xc.TxSignature{},
	}
	err = tx1.AddSignatures([]xc.TxSignature{}...)
	require.EqualError(t, err, "transaction already signed")

	tx2 := tx.Tx{
		TxEnvelope: &xdr.TransactionEnvelope{
			Type: xdr.EnvelopeTypeEnvelopeTypeTx,
			V1: &xdr.TransactionV1Envelope{
				Tx: xdr.Transaction{
					SourceAccount: xdr.MustMuxedAddress("GDLO3EPTGZIC75YG3F3STV5LKUQ6EMGDSNJ4U6JXFUVR7QRZ5KTSYRJF"),
				},
			},
		},
	}
	err = tx2.AddSignatures([]xc.TxSignature{{1, 2, 3}}...)
	require.NoError(t, err)
}
