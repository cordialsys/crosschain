package tx_test

import (
	"encoding/hex"
	"testing"

	xc "github.com/cordialsys/crosschain"
	"github.com/cordialsys/crosschain/chain/hedera/tx"
	"github.com/cordialsys/hedera-protobufs-go/common"
	"github.com/cordialsys/hedera-protobufs-go/services"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
)

func newTx() *tx.Tx {
	cryptoBody := &services.TransactionBody_CryptoTransfer{
		CryptoTransfer: &services.CryptoTransferTransactionBody{
			Transfers: &common.TransferList{
				AccountAmounts: []*common.AccountAmount{
					{
						AccountID: &common.AccountID{
							ShardNum: 0,
							RealmNum: 0,
							Account: &common.AccountID_AccountNum{
								AccountNum: 2025,
							},
						},
						Amount:     -100_000_000,
						IsApproval: false,
					},
					{
						AccountID: &common.AccountID{
							ShardNum: 0,
							RealmNum: 0,
							Account: &common.AccountID_AccountNum{
								AccountNum: 2026,
							},
						},
						Amount:     100_000_000,
						IsApproval: false,
					},
				},
			},
			TokenTransfers: []*common.TokenTransferList{},
		},
	}

	body := &services.TransactionBody{
		TransactionID: &common.TransactionID{
			TransactionValidStart: &common.Timestamp{
				Seconds: 1763124163,
				Nanos:   949083100,
			},
			AccountID: &common.AccountID{
				ShardNum: 0,
				RealmNum: 0,
				Account: &common.AccountID_AccountNum{
					AccountNum: 2025,
				},
			},
		},
		NodeAccountID: &common.AccountID{
			ShardNum: 0,
			RealmNum: 0,
			Account: &common.AccountID_AccountNum{
				AccountNum: 3,
			},
		},
		TransactionFee: 640_000,
		TransactionValidDuration: &services.Duration{
			Seconds: 180,
		},
		Memo: "some memo",
		Data: cryptoBody,
	}
	bodyBytes, err := proto.Marshal(body)
	if err != nil {
		panic(err)
	}

	signature, err := hex.DecodeString("2b52a0e733db28b68aa3aca59abb888d9ca3cd558e2548683f0151391385e55a3b7b9fe6f137fbb512998156994c6b5c402eabc41e9dd1f5adc3c79db20796a401")
	if err != nil {
		panic(err)
	}
	return &tx.Tx{
		SignedTx: &services.SignedTransaction{
			BodyBytes: bodyBytes,
			SigMap: &common.SignatureMap{
				SigPair: []*common.SignaturePair{
					{
						Signature: &common.SignaturePair_ECDSASecp256K1{
							ECDSASecp256K1: signature,
						},
					},
				},
			},
		},
	}
}

func TestTxHash(t *testing.T) {
	tx := newTx()
	require.Equal(t, xc.TxHash("0x4a1faf889e5e09533801d211151c8e661ea180864675c950905b2edd7aeec86b579f47dea888be4d4b23cdb4b38e8fd2"), tx.Hash())
}

func TestTxSighashes(t *testing.T) {
	tx := newTx()
	sighashes, err := tx.Sighashes()
	require.NotNil(t, sighashes)
	require.NoError(t, err)

	require.Len(t, sighashes, 1)

	expectedSighash := []byte{0xb9, 0x23, 0x9, 0x79, 0xcb, 0xfd, 0x72, 0x84, 0x5a, 0xde, 0xde, 0x80, 0xfc, 0xe6, 0x4c, 0x52, 0x30, 0x16, 0x66, 0x8c, 0x7a, 0x66, 0x23, 0x5b, 0x9a, 0x41, 0x6b, 0xed, 0x25, 0x29, 0x0, 0x29}
	require.Equal(t, expectedSighash, sighashes[0].Payload)
}

func TestTxAddSignature(t *testing.T) {
	tx := newTx()
	tx.SignedTx.SigMap = nil
	signature, err := hex.DecodeString("2b52a0e733db28b68aa3aca59abb888d9ca3cd558e2548683f0151391385e55a3b7b9fe6f137fbb512998156994c6b5c402eabc41e9dd1f5adc3c79db20796a401")
	require.NoError(t, err)
	pk, err := hex.DecodeString("04a1d9f1096dc56d52520ad3eee17d675ffbed1eec0222b7ff5c1db2e3edbe1e178186cc5da5c438aec0be028fc8d4284c687903dbe440cbb947882f6d9c661444")
	require.NoError(t, err)
	err = tx.SetSignatures([]*xc.SignatureResponse{
		{
			Signature: xc.TxSignature(signature),
			PublicKey: pk,
		},
	}...)
	require.NoError(t, err)
}

func TestAlreadySigned(t *testing.T) {
	tx := newTx()
	signature, err := hex.DecodeString("2b52a0e733db28b68aa3aca59abb888d9ca3cd558e2548683f0151391385e55a3b7b9fe6f137fbb512998156994c6b5c402eabc41e9dd1f5adc3c79db20796a401")
	require.NoError(t, err)
	err = tx.SetSignatures([]*xc.SignatureResponse{
		{
			Signature: xc.TxSignature(signature),
		},
	}...)
	require.Error(t, err) // already signed
}

func TestEmptySigs(t *testing.T) {
	tx := newTx()
	err := tx.SetSignatures([]*xc.SignatureResponse{}...)
	require.Error(t, err) // already signed
}

func TestInavlidSigCount(t *testing.T) {
	tx := newTx()
	signature, err := hex.DecodeString("2b52a0e733db28b68aa3aca59abb888d9ca3cd558e2548683f0151391385e55a3b7b9fe6f137fbb512998156994c6b5c402eabc41e9dd1f5adc3c79db20796a401")
	require.NoError(t, err)
	err = tx.SetSignatures([]*xc.SignatureResponse{
		{
			Signature: xc.TxSignature(signature),
		},
		{
			Signature: xc.TxSignature(signature),
		},
	}...)
	require.Error(t, err) // already signed
}
