package tx_test

import (
	"testing"

	xc "github.com/cordialsys/crosschain"
	"github.com/cordialsys/crosschain/builder"
	"github.com/cordialsys/crosschain/chain/dusk/tx"
	"github.com/cordialsys/crosschain/chain/dusk/tx_input"
	"github.com/test-go/testify/require"
)

func TestTxHash(t *testing.T) {
	from := xc.Address("2293LeWtYGpsBA99HRg2AfMm9oYhikZ83GSW5NP6QtQxDvkBTAdU8LfQj9fXvDt1rK1baqBcf3gQKsLXpw3LUjpdkSMRMrTsfuTo5Yri1xvUDnVcMMpgTG4o7ThCjZuLMp9L")
	to := xc.Address("26nbWp93it1FF8ChyBUmV2zrXMqsv6xR41UUfcyq37abhoYvvEW4C8MgJPdKnzfQhfa6t1VtVj2QUeDK1aP98TGGtumV897Gtv3M7mh2qZBNK6C4LqvP6GyTeHvC7kPncVvg")
	args, err := builder.NewTransferArgs(
		from,
		to,
		xc.NewAmountBlockchainFromUint64(5_000_000),
	)
	require.NoError(t, err)

	tx, err := tx.NewTx(args, tx_input.TxInput{
		TxInputEnvelope: xc.TxInputEnvelope{},
		Nonce:           10,
		GasLimit:        2_500_000,
		GasPrice:        1,
		RefundAccount:   from,
		Memo:            "",
		ChainId:         1,
	})
	require.NoError(t, err)
	tx.Signature = []byte{138, 52, 141, 88, 247, 205, 110, 26, 136, 4, 115, 92, 9, 180, 157, 74, 111, 167, 81, 176, 40, 192, 82, 165, 224, 187, 10, 48, 123, 54, 6, 103, 91, 40, 171, 11, 228, 111, 194, 56, 33, 140, 131, 4, 134, 17, 126, 228}

	require.Equal(t, xc.TxHash("5a9e2b509d1d3bc3fd1479f09671ac80466e63c70297030590086a6cf088331e"), tx.Hash())
}

func TestTxSighashes(t *testing.T) {

	tx1 := tx.Tx{}
	sighashes, err := tx1.Sighashes()
	require.NotNil(t, sighashes)
	require.EqualError(t, err, "not implemented")
}

func TestTxAddSignature(t *testing.T) {

	tx1 := tx.Tx{}
	err := tx1.AddSignatures([]xc.TxSignature{}...)
	require.EqualError(t, err, "not implemented")
}
