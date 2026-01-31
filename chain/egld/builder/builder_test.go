package builder_test

import (
	"encoding/hex"
	"encoding/json"
	"math/big"
	"strings"
	"testing"

	xc "github.com/cordialsys/crosschain"
	xcbuilder "github.com/cordialsys/crosschain/builder"
	"github.com/cordialsys/crosschain/chain/egld/builder"
	"github.com/cordialsys/crosschain/chain/egld/tx"
	"github.com/cordialsys/crosschain/chain/egld/tx_input"
	"github.com/stretchr/testify/require"
)

func TestNewTxBuilder(t *testing.T) {
	require := require.New(t)

	cfg := &xc.ChainBaseConfig{
		Chain: xc.EGLD,
	}

	txBuilder, err := builder.NewTxBuilder(cfg)
	require.NoError(err)
	require.NotNil(txBuilder)
	require.Equal(xc.EGLD, txBuilder.Asset.Chain)
}

func TestNewNativeTransfer(t *testing.T) {
	require := require.New(t)

	// Create builder
	cfg := &xc.ChainBaseConfig{
		Chain: xc.EGLD,
	}
	txBuilder, err := builder.NewTxBuilder(cfg)
	require.NoError(err)

	// Create TxInput
	input := tx_input.NewTxInput()
	input.Nonce = 42
	input.GasLimit = 50000
	input.GasPrice = 1000000000
	input.ChainID = "1"
	input.Version = 1

	// Create transfer args
	from := xc.Address("erd1r44w4rky0l29pynkp4hrmrjdhnmd5knrrmevarp6h2dg9cu74sas597hhl")
	to := xc.Address("erd1qyu5wthldzr8wx5c9ucg8kjagg0jfs53s8nr3zpz3hypefsdd8ssycr6th")
	amount := xc.NewAmountBlockchainFromStr("1000000000000000000") // 1 EGLD

	args, err := xcbuilder.NewTransferArgs(cfg, from, to, amount)
	require.NoError(err)

	// Build transaction
	txObj, err := txBuilder.NewNativeTransfer(args, input)
	require.NoError(err)
	require.NotNil(txObj)

	// Verify transaction fields
	egldTx, ok := txObj.(*tx.Tx)
	require.True(ok, "transaction should be *tx.Tx type")

	require.Equal(uint64(42), egldTx.Nonce)
	require.Equal("1000000000000000000", egldTx.Value)
	require.Equal(string(to), egldTx.Receiver)
	require.Equal(string(from), egldTx.Sender)
	require.Equal(uint64(1000000000), egldTx.GasPrice)
	require.Equal(uint64(50000), egldTx.GasLimit)
	require.Equal("1", egldTx.ChainID)
	require.Equal(uint32(1), egldTx.Version)
	require.Empty(egldTx.Data)
}

func TestNewNativeTransferSmallAmount(t *testing.T) {
	require := require.New(t)

	cfg := &xc.ChainBaseConfig{
		Chain: xc.EGLD,
	}
	txBuilder, err := builder.NewTxBuilder(cfg)
	require.NoError(err)

	input := tx_input.NewTxInput()
	input.Nonce = 1
	input.GasLimit = 50000
	input.GasPrice = 1000000000
	input.ChainID = "1"
	input.Version = 1

	from := xc.Address("erd1sender")
	to := xc.Address("erd1receiver")
	amount := xc.NewAmountBlockchainFromUint64(1) // 1 wei

	args, err := xcbuilder.NewTransferArgs(cfg, from, to, amount)
	require.NoError(err)

	txObj, err := txBuilder.NewNativeTransfer(args, input)
	require.NoError(err)

	egldTx := txObj.(*tx.Tx)
	require.Equal("1", egldTx.Value)
}

func TestNewTokenTransfer(t *testing.T) {
	require := require.New(t)

	cfg := &xc.ChainBaseConfig{
		Chain: xc.EGLD,
	}
	txBuilder, err := builder.NewTxBuilder(cfg)
	require.NoError(err)

	input := tx_input.NewTxInput()
	input.Nonce = 10
	input.GasLimit = 500000
	input.GasPrice = 1000000000
	input.ChainID = "1"
	input.Version = 1

	from := xc.Address("erd1sender")
	to := xc.Address("erd1receiver")
	amount := xc.NewAmountBlockchainFromUint64(1000000) // 1 USDC (6 decimals)
	contract := xc.ContractAddress("USDC-c76f1f")

	args, err := xcbuilder.NewTransferArgs(cfg, from, to, amount, xcbuilder.OptionContractAddress(contract))
	require.NoError(err)

	txObj, err := txBuilder.NewTokenTransfer(args, contract, input)
	require.NoError(err)
	require.NotNil(txObj)

	egldTx := txObj.(*tx.Tx)
	require.Equal(uint64(10), egldTx.Nonce)
	require.Equal("0", egldTx.Value) // Token transfers have 0 native value
	require.Equal(string(to), egldTx.Receiver)
	require.Equal(string(from), egldTx.Sender)
	require.Equal(uint64(1000000000), egldTx.GasPrice)
	require.Equal(uint64(500000), egldTx.GasLimit)
	require.Equal("1", egldTx.ChainID)
	require.Equal(uint32(1), egldTx.Version)

	// Verify data field
	require.NotEmpty(egldTx.Data)
	dataStr := string(egldTx.Data)

	// Should be: ESDTTransfer@<token_hex>@<amount_hex>
	require.True(strings.HasPrefix(dataStr, "ESDTTransfer@"))

	// Parse the data
	parts := strings.Split(dataStr, "@")
	require.Len(parts, 3)
	require.Equal("ESDTTransfer", parts[0])

	// Verify token identifier hex encoding
	tokenBytes, err := hex.DecodeString(parts[1])
	require.NoError(err)
	require.Equal("USDC-c76f1f", string(tokenBytes))

	// Verify amount hex encoding
	amountBytes, err := hex.DecodeString(parts[2])
	require.NoError(err)
	amountDecoded := new(big.Int).SetBytes(amountBytes)
	require.Equal(int64(1000000), amountDecoded.Int64())
}

func TestNewTokenTransferLargeAmount(t *testing.T) {
	require := require.New(t)

	cfg := &xc.ChainBaseConfig{
		Chain: xc.EGLD,
	}
	txBuilder, err := builder.NewTxBuilder(cfg)
	require.NoError(err)

	input := tx_input.NewTxInput()
	input.Nonce = 5
	input.GasLimit = 500000
	input.GasPrice = 1000000000
	input.ChainID = "D"
	input.Version = 1

	from := xc.Address("erd1sender")
	to := xc.Address("erd1receiver")
	// Large amount: 1 million tokens with 18 decimals = 10^24
	amount := xc.NewAmountBlockchainFromStr("1000000000000000000000000")
	contract := xc.ContractAddress("WEGLD-bd4d79")

	args, err := xcbuilder.NewTransferArgs(cfg, from, to, amount, xcbuilder.OptionContractAddress(contract))
	require.NoError(err)

	txObj, err := txBuilder.NewTokenTransfer(args, contract, input)
	require.NoError(err)

	egldTx := txObj.(*tx.Tx)
	require.Equal("0", egldTx.Value)

	// Verify data field
	dataStr := string(egldTx.Data)
	parts := strings.Split(dataStr, "@")
	require.Len(parts, 3)

	// Verify amount is correctly encoded
	amountBytes, err := hex.DecodeString(parts[2])
	require.NoError(err)
	amountDecoded := new(big.Int).SetBytes(amountBytes)
	expectedAmount, _ := new(big.Int).SetString("1000000000000000000000000", 10)
	require.Equal(expectedAmount, amountDecoded)
}

func TestTransferDispatch(t *testing.T) {
	require := require.New(t)

	cfg := &xc.ChainBaseConfig{
		Chain: xc.EGLD,
	}
	txBuilder, err := builder.NewTxBuilder(cfg)
	require.NoError(err)

	input := tx_input.NewTxInput()
	input.Nonce = 1
	input.GasLimit = 50000
	input.GasPrice = 1000000000
	input.ChainID = "1"
	input.Version = 1

	from := xc.Address("erd1sender")
	to := xc.Address("erd1receiver")
	amount := xc.NewAmountBlockchainFromUint64(1000)

	// Test native transfer dispatch
	argsNative, err := xcbuilder.NewTransferArgs(cfg, from, to, amount)
	require.NoError(err)

	txNative, err := txBuilder.Transfer(argsNative, input)
	require.NoError(err)
	egldTxNative := txNative.(*tx.Tx)
	require.Equal("1000", egldTxNative.Value)
	require.Empty(egldTxNative.Data)

	// Test token transfer dispatch
	contract := xc.ContractAddress("USDC-c76f1f")
	argsToken, err := xcbuilder.NewTransferArgs(cfg, from, to, amount, xcbuilder.OptionContractAddress(contract))
	require.NoError(err)

	input.GasLimit = 500000 // Update gas limit for token transfer
	txToken, err := txBuilder.Transfer(argsToken, input)
	require.NoError(err)
	egldTxToken := txToken.(*tx.Tx)
	require.Equal("0", egldTxToken.Value)
	require.NotEmpty(egldTxToken.Data)
	require.Contains(string(egldTxToken.Data), "ESDTTransfer")
}

func TestBuiltTransactionCanBeSigned(t *testing.T) {
	require := require.New(t)

	cfg := &xc.ChainBaseConfig{
		Chain: xc.EGLD,
	}
	txBuilder, err := builder.NewTxBuilder(cfg)
	require.NoError(err)

	input := tx_input.NewTxInput()
	input.Nonce = 42
	input.GasLimit = 50000
	input.GasPrice = 1000000000
	input.ChainID = "1"
	input.Version = 1

	from := xc.Address("erd1r44w4rky0l29pynkp4hrmrjdhnmd5knrrmevarp6h2dg9cu74sas597hhl")
	to := xc.Address("erd1qyu5wthldzr8wx5c9ucg8kjagg0jfs53s8nr3zpz3hypefsdd8ssycr6th")
	amount := xc.NewAmountBlockchainFromStr("1000000000000000000")

	args, err := xcbuilder.NewTransferArgs(cfg, from, to, amount)
	require.NoError(err)

	txObj, err := txBuilder.NewNativeTransfer(args, input)
	require.NoError(err)

	// Get sighashes
	sighashes, err := txObj.Sighashes()
	require.NoError(err)
	require.Len(sighashes, 1)

	// Verify it's valid JSON
	var txJson map[string]interface{}
	err = json.Unmarshal(sighashes[0].Payload, &txJson)
	require.NoError(err)

	// Verify fields match what we built
	require.Equal(float64(42), txJson["nonce"])
	require.Equal("1000000000000000000", txJson["value"])
	require.Equal(string(to), txJson["receiver"])
	require.Equal(string(from), txJson["sender"])

	// Add a signature
	signatureBytes := make([]byte, 64)
	for i := range signatureBytes {
		signatureBytes[i] = byte(i)
	}
	sig := &xc.SignatureResponse{
		Signature: signatureBytes,
	}

	err = txObj.SetSignatures(sig)
	require.NoError(err)

	// Verify signature was set
	egldTx := txObj.(*tx.Tx)
	require.NotEmpty(egldTx.Signature)

	// Verify transaction can be serialized
	serialized, err := txObj.Serialize()
	require.NoError(err)
	require.NotEmpty(serialized)

	// Verify serialized is valid JSON
	var serializedJson map[string]interface{}
	err = json.Unmarshal(serialized, &serializedJson)
	require.NoError(err)
	require.Contains(serializedJson, "signature")
}
