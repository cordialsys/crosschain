package substrate_test

import (
	"encoding/json"

	"github.com/centrifuge/go-substrate-rpc-client/v4/types"
	"github.com/centrifuge/go-substrate-rpc-client/v4/types/codec"
	xc "github.com/cordialsys/crosschain"
	"github.com/cordialsys/crosschain/chain/substrate"
)

func (s *CrosschainTestSuite) TestNewTxBuilder() {
	require := s.Require()
	builder, err := substrate.NewTxBuilder(&xc.ChainConfig{})
	require.NoError(err)
	require.NotNil(builder)
}

func (s *CrosschainTestSuite) TestNewTransferFail() {
	require := s.Require()
	builder, _ := substrate.NewTxBuilder(&xc.ChainConfig{})
	from := xc.Address("5GL7deqCmoKpgmhq3b12DXSAu62VQ3DCqN3Z7Bet6fx9qAyb")
	to := xc.Address("5FUh5YJztrDvQe58YcDr5rDhkx1kSZcxQFu81wamrPuVyZSW")
	amount := xc.NewAmountBlockchainFromUint64(10000000000)
	input := &TxInput{} // missing metadata
	_, err := builder.NewTransfer(from, to, amount, input)
	require.Error(err)
}

func (s *CrosschainTestSuite) TestNewTransfer() {
	require := s.Require()
	builder, _ := substrate.NewTxBuilder(&xc.ChainConfig{Decimals: 10})
	from := xc.Address("5GL7deqCmoKpgmhq3b12DXSAu62VQ3DCqN3Z7Bet6fx9qAyb")
	to := xc.Address("5FUh5YJztrDvQe58YcDr5rDhkx1kSZcxQFu81wamrPuVyZSW")
	amount := xc.NewAmountBlockchainFromUint64(10000000000)
	inputBz := `{
		"type":"substrate",
		"meta":{"calls":[{"name":"Balances.transfer_keep_alive","section":4,"method":3}],"signed_extensions":["CheckNonZeroSender","CheckSpecVersion","CheckTxVersion","CheckGenesis","CheckMortality","CheckNonce","CheckWeight","ChargeTransactionPayment","CheckMetadataHash"]},
		"genesis_hash":"0x6408de7737c59c238890533af25896a2c20608d8b380bb01029acb392781063e",
		"current_hash":"0xb81dc05b7ba338a8eb0abe724d77fc9ac7af2e7b951e7605bcc27957156a4934",
		"runtime_version":{"apis":[["0xdf6acb689907609b",5],["0x6ff52ee858e6c5bd",1],["0x91b1c8b16328eb92",1],["0x9ffb505aa738d69c",1],["0x37e397fc7c91f5e4",2],["0x40fe3ad401f8959a",6],["0xd2bc9897eed08f15",3],["0xf78b278be53f454c",2],["0xaf2c0297a23e6d3d",11],["0x49eaaf1b548a0cb0",3],["0x91d5df18b0d2cf58",2],["0xed99c5acb25eedf5",3],["0xcbca25e39f142387",2],["0x687ad44ad37f03c2",1],["0xab3c0572291feb8b",1],["0xbc9d89904f5b923f",1],["0x37c8bb1350a9a2a8",4],["0x2a5e924655399e60",1],["0xfbc577b9d747efd6",1]],
		"authoringVersion":0,"implName":"parity-rococo-v2.0","implVersion":0,"specName":"rococo","specVersion":1014000,"transactionVersion":26},
		"current_height":11347363,
		"account_nonce":14,
		"tip":100}
	`
	input := &TxInput{}
	err := json.Unmarshal([]byte(inputBz), input)
	require.NoError(err)
	tx, err := builder.NewTransfer(from, to, amount, input)
	require.NoError(err)
	require.NotNil(tx)

	err = tx.AddSignatures(make([]byte, 64))
	require.NoError(err)

	require.NotEmpty(tx.Hash())

	bz, err := tx.Serialize()
	require.NoError(err)
	require.NotEmpty(bz)

	ext := types.Extrinsic{}
	err = codec.Decode(bz, &ext)
	require.NoError(err)

	require.EqualValues(true, ext.IsSigned())
	require.EqualValues(14, ext.Signature.Nonce.Int64())
	require.EqualValues(100, ext.Signature.Tip.Int64())
}
