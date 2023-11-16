package cosmos

import (
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"sort"

	xc "github.com/cordialsys/crosschain"
	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/types"
	signingtypes "github.com/cosmos/cosmos-sdk/types/tx/signing"
	"github.com/cosmos/cosmos-sdk/x/auth/signing"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"

	wasmtypes "github.com/CosmWasm/wasmd/x/wasm/types"
)

var NativeTransferGasLimit = uint64(400_000)
var TokenTransferGasLimit = uint64(900_000)
var DefaultMaxTotalFeeHuman, _ = xc.NewAmountHumanReadableFromStr("2")

// TxBuilder for Cosmos
type TxBuilder struct {
	xc.TxBuilder
	Asset           xc.ITask
	CosmosTxConfig  client.TxConfig
	CosmosTxBuilder client.TxBuilder
}

// NewTxBuilder creates a new Cosmos TxBuilder
func NewTxBuilder(asset xc.ITask) (xc.TxBuilder, error) {
	cosmosCfg := MakeCosmosConfig()

	return TxBuilder{
		Asset:           asset,
		CosmosTxConfig:  cosmosCfg.TxConfig,
		CosmosTxBuilder: cosmosCfg.TxConfig.NewTxBuilder(),
	}, nil
}

func DefaultMaxGasPrice(nativeAsset *xc.ChainConfig) float64 {
	// Don't spend more than e.g. 2 LUNA on a transaction
	maxFee := DefaultMaxTotalFeeHuman.ToBlockchain(nativeAsset.Decimals)
	return TotalFeeToFeePerGas(maxFee.String(), NativeTransferGasLimit)
}

// NewTransfer creates a new transfer for an Asset, either native or token
func (txBuilder TxBuilder) NewTransfer(from xc.Address, to xc.Address, amount xc.AmountBlockchain, input xc.TxInput) (xc.Tx, error) {
	txInput := input.(*TxInput)
	native := txBuilder.Asset.GetChain()
	max := native.ChainMaxGasPrice
	if max <= 0 {
		max = DefaultMaxGasPrice(native)
	}
	// enforce a maximum gas price
	if txInput.GasPrice > max {
		txInput.GasPrice = max
	}

	// cosmos is unique in that:
	// - the native asset is in one of the native modules, x/bank
	// - x/bank can have multiple assets, all of which can typically pay for gas
	//   - this means cosmos has "multiple" native assets and can add more later, similar to tokens.
	// - there can be other modules with tokens, like cw20 in x/wasm.
	// To abstract this, we detect the module for the asset and rely on that for the transfer types.
	// A native transfer can be a token transfer and vice versa.
	// Right now gas is always paid in the "default" gas coin, set by config.

	// because cosmos assets can be transferred via a number of different modules, we have to rely on txInput
	// to determine which cosmos module we should
	switch txInput.AssetType {
	case BANK:
		return txBuilder.NewBankTransfer(from, to, amount, input)
	case CW20:
		return txBuilder.NewCW20Transfer(from, to, amount, input)
	default:
		return nil, errors.New("unknown cosmos asset type: " + string(txInput.AssetType))
	}
}

// See NewTransfer
func (txBuilder TxBuilder) NewNativeTransfer(from xc.Address, to xc.Address, amount xc.AmountBlockchain, input xc.TxInput) (xc.Tx, error) {
	return txBuilder.NewTransfer(from, to, amount, input)
}

// See NewTransfer
func (txBuilder TxBuilder) NewTokenTransfer(from xc.Address, to xc.Address, amount xc.AmountBlockchain, input xc.TxInput) (xc.Tx, error) {
	return txBuilder.NewTransfer(from, to, amount, input)
}

// x/bank MsgSend transfer
func (txBuilder TxBuilder) NewBankTransfer(from xc.Address, to xc.Address, amount xc.AmountBlockchain, input xc.TxInput) (xc.Tx, error) {
	txInput := input.(*TxInput)
	amountInt := big.Int(amount)

	if txInput.GasLimit == 0 {
		txInput.GasLimit = NativeTransferGasLimit
	}

	denom := txBuilder.GetDenom()
	msgSend := &banktypes.MsgSend{
		FromAddress: string(from),
		ToAddress:   string(to),
		Amount: types.Coins{
			{
				Denom:  denom,
				Amount: types.NewIntFromBigInt(&amountInt),
			},
		},
	}

	return txBuilder.createTxWithMsg(from, to, amount, txInput, msgSend)
}

func (txBuilder TxBuilder) NewCW20Transfer(from xc.Address, to xc.Address, amount xc.AmountBlockchain, input xc.TxInput) (xc.Tx, error) {
	txInput := input.(*TxInput)
	asset := txBuilder.Asset

	if txInput.GasLimit == 0 {
		txInput.GasLimit = TokenTransferGasLimit
	}
	contract := asset.GetContract()
	contractTransferMsg := fmt.Sprintf(`{"transfer": {"amount": "%s", "recipient": "%s"}}`, amount.String(), to)
	msgSend := &wasmtypes.MsgExecuteContract{
		Sender:   string(from),
		Contract: contract,
		Msg:      wasmtypes.RawContractMessage(json.RawMessage(contractTransferMsg)),
	}

	return txBuilder.createTxWithMsg(from, to, amount, txInput, msgSend)
}

func (txBuilder TxBuilder) GetDenom() string {
	asset := txBuilder.Asset
	denom := asset.GetChain().ChainCoin
	if token, ok := asset.(*xc.TokenAssetConfig); ok {
		if token.Contract != "" {
			denom = token.Contract
		}
	}
	return denom
}

// Returns the amount in blockchain that is percentage of amount.
// E.g. amount = 100, tax = 0.05, returns 5.
func GetTaxFrom(amount xc.AmountBlockchain, tax float64) xc.AmountBlockchain {
	if tax > 0.00001 {
		precisionInt := uint64(10000000)
		taxBig := xc.NewAmountBlockchainFromUint64(uint64(float64(precisionInt) * tax))
		// some chains may implement a tax (terra classic)
		product := amount.Mul(&taxBig).Int()
		quotiant := product.Div(product, big.NewInt(int64(precisionInt)))
		return xc.NewAmountBlockchainFromStr(quotiant.String())
	}
	return xc.NewAmountBlockchainFromUint64(0)
}

// createTxWithMsg creates a new Tx given Cosmos Msg
func (txBuilder TxBuilder) createTxWithMsg(from xc.Address, to xc.Address, amount xc.AmountBlockchain, input *TxInput, msg types.Msg) (xc.Tx, error) {
	asset := txBuilder.Asset
	cosmosTxConfig := txBuilder.CosmosTxConfig
	cosmosBuilder := txBuilder.CosmosTxBuilder

	err := cosmosBuilder.SetMsgs(msg)
	if err != nil {
		return nil, err
	}

	gasDenom := asset.GetChain().GasCoin
	if gasDenom == "" {
		gasDenom = asset.GetChain().ChainCoin
	}
	cosmosBuilder.SetMemo(input.Memo)
	cosmosBuilder.SetGasLimit(input.GasLimit)
	feeCoins := types.Coins{
		{
			Denom:  gasDenom,
			Amount: types.NewIntFromUint64(uint64(input.GasPrice * float64(input.GasLimit))),
		},
	}
	taxRate := txBuilder.Asset.GetChain().ChainTransferTax
	tax := GetTaxFrom(amount, taxRate)
	if tax.Uint64() > 0 {
		taxDenom := asset.GetChain().ChainCoin
		if token, ok := asset.(*xc.TokenAssetConfig); ok && token.Contract != "" {
			taxDenom = token.Contract
		}
		taxStr, _ := types.NewIntFromString(tax.String())
		// cannot add two coins that are the same so must check
		if feeCoins[0].Denom == taxDenom {
			// add to existing
			feeCoins[0].Amount = feeCoins[0].Amount.Add(taxStr)
		} else {
			// add new
			feeCoins = append(feeCoins, types.Coin{
				Denom:  taxDenom,
				Amount: taxStr,
			})
		}
	}
	// Must be sorted or cosmos client panics
	sort.Slice(feeCoins, func(i, j int) bool {
		return feeCoins[i].Denom < feeCoins[j].Denom
	})
	cosmosBuilder.SetFeeAmount(feeCoins)

	sigMode := signingtypes.SignMode_SIGN_MODE_DIRECT
	sigsV2 := []signingtypes.SignatureV2{
		{
			PubKey: getPublicKey(asset.GetChain(), input.FromPublicKey),
			Data: &signingtypes.SingleSignatureData{
				SignMode:  sigMode,
				Signature: nil,
			},
			Sequence: input.Sequence,
		},
	}
	err = cosmosBuilder.SetSignatures(sigsV2...)
	if err != nil {
		return nil, err
	}

	signerData := signing.SignerData{
		AccountNumber: input.AccountNumber,
		ChainID:       asset.GetChain().ChainIDStr,
		Sequence:      input.Sequence,
	}
	sighashData, err := cosmosTxConfig.SignModeHandler().GetSignBytes(sigMode, signerData, cosmosBuilder.GetTx())
	if err != nil {
		return nil, err
	}
	sighash := getSighash(asset.GetChain(), sighashData)
	return &Tx{
		CosmosTx:        cosmosBuilder.GetTx(),
		ParsedTransfers: []types.Msg{msg},
		CosmosTxBuilder: cosmosBuilder,
		CosmosTxEncoder: cosmosTxConfig.TxEncoder(),
		SigsV2:          sigsV2,
		TxDataToSign:    sighash,
	}, nil
}
