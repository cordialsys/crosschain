package tx_test

import (
	"encoding/hex"
	"testing"

	xc "github.com/cordialsys/crosschain"
	"github.com/cordialsys/crosschain/builder"
	"github.com/cordialsys/crosschain/chain/dusk/tx"
	"github.com/cordialsys/crosschain/chain/dusk/tx_input"
	"github.com/stretchr/testify/require"
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
		ChainId:         1,
	})
	require.NoError(t, err)
	tx.Signature = []byte{138, 52, 141, 88, 247, 205, 110, 26, 136, 4, 115, 92, 9, 180, 157, 74, 111, 167, 81, 176, 40, 192, 82, 165, 224, 187, 10, 48, 123, 54, 6, 103, 91, 40, 171, 11, 228, 111, 194, 56, 33, 140, 131, 4, 134, 17, 126, 228}

	require.Equal(t, xc.TxHash("5a9e2b509d1d3bc3fd1479f09671ac80466e63c70297030590086a6cf088331e"), tx.Hash())
}

func TestTxMemoHash(t *testing.T) {
	from := xc.Address("2293LeWtYGpsBA99HRg2AfMm9oYhikZ83GSW5NP6QtQxDvkBTAdU8LfQj9fXvDt1rK1baqBcf3gQKsLXpw3LUjpdkSMRMrTsfuTo5Yri1xvUDnVcMMpgTG4o7ThCjZuLMp9L")
	to := xc.Address("26nbWp93it1FF8ChyBUmV2zrXMqsv6xR41UUfcyq37abhoYvvEW4C8MgJPdKnzfQhfa6t1VtVj2QUeDK1aP98TGGtumV897Gtv3M7mh2qZBNK6C4LqvP6GyTeHvC7kPncVvg")
	args, err := builder.NewTransferArgs(
		from,
		to,
		xc.NewAmountBlockchainFromUint64(5_000_000),
		// test memo serialization
		builder.OptionMemo("1234"),
	)
	require.NoError(t, err)

	tx, err := tx.NewTx(args, tx_input.TxInput{
		TxInputEnvelope: xc.TxInputEnvelope{},
		Nonce:           10,
		GasLimit:        2_500_000,
		GasPrice:        1,
		RefundAccount:   from,
		ChainId:         1,
	})
	require.NoError(t, err)
	tx.Signature = []byte{138, 52, 141, 88, 247, 205, 110, 26, 136, 4, 115, 92, 9, 180, 157, 74, 111, 167, 81, 176, 40, 192, 82, 165, 224, 187, 10, 48, 123, 54, 6, 103, 91, 40, 171, 11, 228, 111, 194, 56, 33, 140, 131, 4, 134, 17, 126, 228}

	require.Equal(t, xc.TxHash("e96ccec717989738880f7dc1bba0cdf63b794d32349e7f2c512f91cfab3d2548"), tx.Hash())

	bz, err := tx.Serialize()
	require.NoError(t, err)
	require.Equal(t,
		"01f80000000000000001abad78acfb289222927578c11cf82afb09847775b9352d416d3cad57d0d978941201606682d9a5a37340033bcb29a4860e42925a3fd21094e0b54a95a595ec4f69a800d08446ded38847a4a1d6ed6a8bd7ef2155aa74c8d4cc57e4945989874f01b92b4e156a05a46c7096e291dcd5e816b9a894f43ee31c3557a1ce9c6d111f75c8b2d40dd30fd75b8f1cc438d9764c8e1486941ecc31c431056eb320bdf8712ce2e2690fab0554f58a62e6d863b9501f126c2fe187e07b7a94907cf96b2137e1404b4c00000000000000000000000000a0252600000000000100000000000000000a00000000000000030400000000000000313233348a348d58f7cd6e1a8804735c09b49d4a6fa751b028c052a5e0bb0a307b3606675b28ab0be46fc238218c830486117ee4",
		hex.EncodeToString(bz),
	)
}

func TestTxAddSignature(t *testing.T) {

	tx1 := tx.Tx{}
	err := tx1.AddSignatures([]xc.TxSignature{}...)
	require.EqualError(t, err, "only one signature is allowed")
}
