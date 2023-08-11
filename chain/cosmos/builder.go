package cosmos

import (
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"sort"
	"strings"

	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/types"
	signingtypes "github.com/cosmos/cosmos-sdk/types/tx/signing"
	"github.com/cosmos/cosmos-sdk/x/auth/signing"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
	xc "github.com/jumpcrypto/crosschain"

	// wasmtypes "github.com/CosmWasm/wasmd/x/wasm/types"
	wasmtypes "github.com/CosmWasm/wasmd/x/wasm/types"
)

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

// NewTransfer creates a new transfer for an Asset, either native or token
func (txBuilder TxBuilder) NewTransfer(from xc.Address, to xc.Address, amount xc.AmountBlockchain, input xc.TxInput) (xc.Tx, error) {
	if isNativeAsset(txBuilder.Asset.GetAssetConfig()) {
		return txBuilder.NewNativeTransfer(from, to, amount, input)
	}
	return txBuilder.NewTokenTransfer(from, to, amount, input)
}

// NewNativeTransfer creates a new transfer for a native asset
func (txBuilder TxBuilder) NewNativeTransfer(from xc.Address, to xc.Address, amount xc.AmountBlockchain, input xc.TxInput) (xc.Tx, error) {
	txInput := input.(*TxInput)
	asset := txBuilder.Asset
	amountInt := big.Int(amount)

	if txInput.GasLimit == 0 {
		txInput.GasLimit = 400_000
	}

	denom := asset.GetNativeAsset().ChainCoin
	if token, ok := asset.(*xc.TokenAssetConfig); ok {
		if token.Contract != "" {
			denom = token.Contract
		}
	}

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

// NewTokenTransfer creates a new transfer for a token asset
func (txBuilder TxBuilder) NewTokenTransfer(from xc.Address, to xc.Address, amount xc.AmountBlockchain, input xc.TxInput) (xc.Tx, error) {
	txInput := input.(*TxInput)
	asset := txBuilder.Asset

	// Terra Classic: most tokens are actually native tokens
	// in crosschain we can treat them interchangeably as native or non-native assets
	// if contract isn't a valid address, they're native tokens
	if isNativeAsset(asset.GetAssetConfig()) {
		return txBuilder.NewNativeTransfer(from, to, amount, input)
	}

	if txInput.GasLimit == 0 {
		txInput.GasLimit = 900_000
	}

	contractTransferMsg := fmt.Sprintf(`{"transfer": {"amount": "%s", "recipient": "%s"}}`, amount.String(), to)
	msgSend := &wasmtypes.MsgExecuteContract{
		Sender:   string(from),
		Contract: asset.GetAssetConfig().Contract,
		Msg:      wasmtypes.RawContractMessage(json.RawMessage(contractTransferMsg)),
	}

	return txBuilder.createTxWithMsg(from, to, amount, txInput, msgSend)
}

func accAddressFromBech32WithPrefix(address string, prefix string) ([]byte, error) {
	if len(strings.TrimSpace(address)) == 0 {
		return nil, errors.New("empty address string is not allowed")
	}

	addressBytes, err := types.GetFromBech32(address, prefix)
	if err != nil {
		return nil, err
	}

	err = types.VerifyAddressFormat(addressBytes)
	if err != nil {
		return nil, err
	}

	return addressBytes, nil
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

	_, err = accAddressFromBech32WithPrefix(string(from), asset.GetNativeAsset().ChainPrefix)
	if err != nil {
		return nil, err
	}

	_, err = accAddressFromBech32WithPrefix(string(to), asset.GetNativeAsset().ChainPrefix)
	if err != nil {
		return nil, err
	}
	gasDenom := asset.GetNativeAsset().GasCoin
	if gasDenom == "" {
		gasDenom = asset.GetNativeAsset().ChainCoin
	}
	cosmosBuilder.SetMemo(input.Memo)
	cosmosBuilder.SetGasLimit(input.GasLimit)
	feeCoins := types.Coins{
		{
			Denom:  gasDenom,
			Amount: types.NewIntFromUint64(uint64(input.GasPrice * float64(input.GasLimit))),
		},
	}
	taxRate := txBuilder.Asset.GetNativeAsset().ChainTransferTax
	tax := GetTaxFrom(amount, taxRate)
	if tax.Uint64() > 0 {
		taxDenom := asset.GetNativeAsset().ChainCoin
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
			PubKey: getPublicKey(*asset.GetNativeAsset(), input.FromPublicKey),
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
		ChainID:       asset.GetNativeAsset().ChainIDStr,
		Sequence:      input.Sequence,
	}
	sighashData, err := cosmosTxConfig.SignModeHandler().GetSignBytes(sigMode, signerData, cosmosBuilder.GetTx())
	if err != nil {
		return nil, err
	}
	sighash := getSighash(*asset.GetNativeAsset(), sighashData)
	return &Tx{
		CosmosTx:        cosmosBuilder.GetTx(),
		ParsedTransfers: []types.Msg{msg},
		CosmosTxBuilder: cosmosBuilder,
		CosmosTxEncoder: cosmosTxConfig.TxEncoder(),
		SigsV2:          sigsV2,
		TxDataToSign:    sighash,
	}, nil
}
