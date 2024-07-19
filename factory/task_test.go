package factory_test

import (
	"encoding/hex"
	"strings"

	xc "github.com/cordialsys/crosschain"
	evmtx "github.com/cordialsys/crosschain/chain/evm/tx"
	evminput "github.com/cordialsys/crosschain/chain/evm/tx_input"
)

type TxInput = evminput.TxInput
type Tx = evmtx.Tx

func (s *CrosschainTestSuite) TestEthWrap() {
	require := s.Require()
	asset, err := s.Factory.GetAssetConfig("WETH", "ETH")
	require.Nil(err)
	require.NotNil(asset)
	task, err := s.Factory.GetTaskConfig("eth-wrap", "ETH")
	require.Nil(err)
	require.NotNil(task)

	txBuilder, err := s.Factory.NewTxBuilder(task)
	require.Nil(err)
	require.NotNil(txBuilder)

	txInput := TxInput{}
	tx, err := txBuilder.NewTransfer("from", "to", xc.NewAmountBlockchainFromUint64(123), &txInput)
	require.Nil(err)
	evmTx := tx.(*Tx).EthTx
	require.Equal(uint8(0x2), evmTx.Type())
	require.Equal(uint64(800_000), evmTx.Gas())

	require.Equal(asset.(*xc.TokenAssetConfig).Contract, evmTx.To().String())
	require.Equal("123", evmTx.Value().String())
	expectedData := "d0e30db0"
	require.Equal(expectedData, hex.EncodeToString(evmTx.Data()))
}

func (s *CrosschainTestSuite) TestEthWrapPassingWETH() {
	require := s.Require()
	asset, err := s.Factory.GetAssetConfig("WETH", "ETH")
	require.Nil(err)
	require.NotNil(asset)
	task, err := s.Factory.GetTaskConfig("eth-wrap", "ETH")
	require.Nil(err)
	require.NotNil(task)

	txBuilder, err := s.Factory.NewTxBuilder(task)
	require.Nil(err)
	require.NotNil(txBuilder)

	txInput := TxInput{}
	tx, err := txBuilder.NewTransfer("from", "to", xc.NewAmountBlockchainFromUint64(123), &txInput)
	require.Nil(err)
	evmTx := tx.(*Tx).EthTx
	require.Equal(uint8(0x2), evmTx.Type())
	require.Equal(uint64(800_000), evmTx.Gas())

	require.Equal(asset.(*xc.TokenAssetConfig).Contract, evmTx.To().String())
	require.Equal("123", evmTx.Value().String())
	expectedData := "d0e30db0"
	require.Equal(expectedData, hex.EncodeToString(evmTx.Data()))
}

func (s *CrosschainTestSuite) TestEthUnwrap() {
	require := s.Require()
	asset, err := s.Factory.GetAssetConfig("WETH", "ETH")
	require.Nil(err)
	require.NotNil(asset)
	task, err := s.Factory.GetTaskConfig("eth-unwrap", "WETH.ETH")
	require.Nil(err)
	require.NotNil(task)

	txBuilder, err := s.Factory.NewTxBuilder(task)
	require.Nil(err)
	require.NotNil(txBuilder)

	txInput := TxInput{}
	tx, err := txBuilder.NewTransfer("from", "to", xc.NewAmountBlockchainFromUint64(0x123), &txInput)
	require.Nil(err)
	evmTx := tx.(*Tx).EthTx
	require.Equal(uint8(0x2), evmTx.Type())
	require.Equal(uint64(800_000), evmTx.Gas())

	require.Equal(asset.(*xc.TokenAssetConfig).Contract, evmTx.To().String())
	require.Equal("0", evmTx.Value().String())
	expectedData := "2e1a7d4d" +
		"0000000000000000000000000000000000000000000000000000000000000123"
	require.Equal(expectedData, hex.EncodeToString(evmTx.Data()))
}

func (s *CrosschainTestSuite) TestERC20Transfer() {
	require := s.Require()
	asset, err := s.Factory.GetAssetConfig("USDC", "ETH")
	require.Nil(err)
	require.NotNil(asset)
	task, err := s.Factory.GetTaskConfig("erc20-transfer", "USDC.ETH")
	require.Nil(err)
	require.NotNil(task)

	txBuilder, err := s.Factory.NewTxBuilder(task)
	require.Nil(err)
	require.NotNil(txBuilder)

	txInput := TxInput{}
	from := "0x0eC9f48533bb2A03F53F341EF5cc1B057892B10B"
	to := "a0a5C02F0371cCc142ad5AD170C291c86c3E6379"
	tx, err := txBuilder.NewTransfer(xc.Address(from), xc.Address(to), xc.NewAmountBlockchainFromUint64(0x123), &txInput)
	require.Nil(err)
	evmTx := tx.(*Tx).EthTx
	require.Equal(uint8(0x2), evmTx.Type())
	require.Equal(uint64(800_000), evmTx.Gas())

	require.Equal(strings.ToLower(asset.(*xc.TokenAssetConfig).Contract), strings.ToLower(evmTx.To().String()))
	require.Equal("0", evmTx.Value().String())
	expectedData := "a9059cbb" +
		"000000000000000000000000" + strings.ToLower(to) +
		"0000000000000000000000000000000000000000000000000000000000000123"
	require.Equal(expectedData, hex.EncodeToString(evmTx.Data()))

	// test that a token transfer produces the same result (except for gas limit)
	txBuilder, err = s.Factory.NewTxBuilder(asset)
	require.Nil(err)
	require.NotNil(txBuilder)

	tx2, err := txBuilder.NewTransfer(xc.Address(from), xc.Address(to), xc.NewAmountBlockchainFromUint64(0x123), &txInput)
	require.Nil(err)
	evmTx2 := tx2.(*Tx).EthTx

	require.Equal(evmTx.To().String(), evmTx2.To().String())
	require.Equal(evmTx.Value(), evmTx2.Value())
	require.Equal(hex.EncodeToString(evmTx.Data()), hex.EncodeToString(evmTx2.Data()))
}
