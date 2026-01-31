package builder

import (
	"encoding/hex"
	"fmt"
	"math/big"

	xc "github.com/cordialsys/crosschain"
	xcbuilder "github.com/cordialsys/crosschain/builder"
	"github.com/cordialsys/crosschain/chain/egld/tx"
	"github.com/cordialsys/crosschain/chain/egld/tx_input"
)

type TxBuilder struct {
	Asset *xc.ChainBaseConfig
}

var _ xcbuilder.FullTransferBuilder = TxBuilder{}

func NewTxBuilder(cfgI *xc.ChainBaseConfig) (TxBuilder, error) {
	return TxBuilder{
		Asset: cfgI,
	}, nil
}

func (txBuilder TxBuilder) Transfer(args xcbuilder.TransferArgs, input xc.TxInput) (xc.Tx, error) {
	if contract, ok := args.GetContract(); ok {
		return txBuilder.NewTokenTransfer(args, contract, input)
	} else {
		return txBuilder.NewNativeTransfer(args, input)
	}
}

func (txBuilder TxBuilder) NewNativeTransfer(args xcbuilder.TransferArgs, input xc.TxInput) (xc.Tx, error) {
	egldInput, ok := input.(*tx_input.TxInput)
	if !ok {
		return nil, fmt.Errorf("invalid input type")
	}

	amountBig := big.Int(args.GetAmount())
	value := amountBig.String()

	txObj := &tx.Tx{
		Nonce:    egldInput.Nonce,
		Value:    value,
		Receiver: string(args.GetTo()),
		Sender:   string(args.GetFrom()),
		GasPrice: egldInput.GasPrice,
		GasLimit: egldInput.GasLimit,
		ChainID:  egldInput.ChainID,
		Version:  egldInput.Version,
	}

	return txObj, nil
}

func (txBuilder TxBuilder) NewTokenTransfer(args xcbuilder.TransferArgs, contract xc.ContractAddress, input xc.TxInput) (xc.Tx, error) {
	egldInput, ok := input.(*tx_input.TxInput)
	if !ok {
		return nil, fmt.Errorf("invalid input type")
	}

	amountBig := big.Int(args.GetAmount())
	amountHex := hex.EncodeToString(amountBig.Bytes())
	if amountHex == "" {
		amountHex = "00"
	}

	tokenHex := hex.EncodeToString([]byte(contract))

	// ESDT transfer data format: ESDTTransfer@<token_hex>@<amount_hex>
	dataStr := fmt.Sprintf("ESDTTransfer@%s@%s", tokenHex, amountHex)

	// ESDT transfers have 0 native value; transfer is encoded in data field
	txObj := &tx.Tx{
		Nonce:    egldInput.Nonce,
		Value:    "0",
		Receiver: string(args.GetTo()),
		Sender:   string(args.GetFrom()),
		GasPrice: egldInput.GasPrice,
		GasLimit: egldInput.GasLimit,
		Data:     []byte(dataStr),
		ChainID:  egldInput.ChainID,
		Version:  egldInput.Version,
	}

	return txObj, nil
}
