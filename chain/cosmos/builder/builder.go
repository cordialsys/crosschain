package builder

import (
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"sort"

	"cosmossdk.io/math"
	banktypes "cosmossdk.io/x/bank/types"
	xc "github.com/cordialsys/crosschain"
	xcbuilder "github.com/cordialsys/crosschain/builder"
	"github.com/cordialsys/crosschain/chain/cosmos/tx"
	"github.com/cordialsys/crosschain/chain/cosmos/tx_input"
	"github.com/cordialsys/crosschain/chain/cosmos/tx_input/gas"
	localcodectypes "github.com/cordialsys/crosschain/chain/cosmos/types"
	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/types"

	wasmtypes "github.com/cordialsys/crosschain/chain/cosmos/types/CosmWasm/wasmd/x/wasm/types"
)

// TxBuilder for Cosmos
type TxBuilder struct {
	Asset          *xc.ChainBaseConfig
	CosmosTxConfig client.TxConfig
}

var _ xcbuilder.FullBuilder = &TxBuilder{}

// NewTxBuilder creates a new Cosmos TxBuilder
func NewTxBuilder(asset *xc.ChainBaseConfig) (TxBuilder, error) {
	cosmosCfg, err := localcodectypes.MakeCosmosConfig(asset)
	if err != nil {
		return TxBuilder{}, err
	}

	return TxBuilder{
		Asset:          asset,
		CosmosTxConfig: cosmosCfg.TxConfig,
	}, nil
}

// func (txBuilder TxBuilder) NewTransfer(from xc.Address, to xc.Address, amount xc.AmountBlockchain, input xc.TxInput) (xc.Tx, error) {
func (txBuilder TxBuilder) Transfer(args xcbuilder.TransferArgs, input xc.TxInput) (xc.Tx, error) {
	txInput := input.(*tx_input.TxInput)
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
	case tx_input.BANK:
		return txBuilder.NewBankTransfer(args, input)
	case tx_input.CW20:
		contract, _ := args.GetContract()
		return txBuilder.NewCW20Transfer(args, contract, input)
	default:
		return nil, errors.New("unknown cosmos asset type: " + string(txInput.AssetType))
	}
}

// x/bank MsgSend transfer
func (txBuilder TxBuilder) NewBankTransfer(args xcbuilder.TransferArgs, input xc.TxInput) (xc.Tx, error) {
	txInput := input.(*tx_input.TxInput)
	amountInt := big.Int(args.GetAmount())

	if txInput.GasLimit == 0 {
		txInput.GasLimit = gas.NativeTransferGasLimit
	}
	contractMaybe, _ := args.GetContract()
	denom := txBuilder.GetDenom(contractMaybe)
	msgSend := &banktypes.MsgSend{
		FromAddress: string(args.GetFrom()),
		ToAddress:   string(args.GetTo()),
		Amount: types.Coins{
			{
				Denom:  denom,
				Amount: math.NewIntFromBigInt(&amountInt),
			},
		},
	}

	fees := txBuilder.calculateFees(args.GetAmount(), contractMaybe, txInput, true)
	return txBuilder.createTxWithMsg(txInput, msgSend, txArgs{
		Memo:          txInput.LegacyMemo,
		FromPublicKey: txInput.LegacyFromPublicKey,
	}, fees)
}

func (txBuilder TxBuilder) NewCW20Transfer(args xcbuilder.TransferArgs, contract xc.ContractAddress, input xc.TxInput) (xc.Tx, error) {
	txInput := input.(*tx_input.TxInput)

	if txInput.GasLimit == 0 {
		txInput.GasLimit = gas.TokenTransferGasLimit
	}
	contractTransferMsg := fmt.Sprintf(`{"transfer": {"amount": "%s", "recipient": "%s"}}`, args.GetAmount().String(), args.GetTo())
	msgSend := &wasmtypes.MsgExecuteContract{
		Sender:   string(args.GetFrom()),
		Contract: string(contract),
		Msg:      wasmtypes.RawContractMessage(json.RawMessage(contractTransferMsg)),
	}

	fees := txBuilder.calculateFees(args.GetAmount(), contract, txInput, false)

	return txBuilder.createTxWithMsg(txInput, msgSend, txArgs{
		Memo:          txInput.LegacyMemo,
		FromPublicKey: txInput.LegacyFromPublicKey,
	}, fees)
}

func (txBuilder TxBuilder) GetDenom(contract xc.ContractAddress) string {
	asset := txBuilder.Asset
	denom := asset.ChainCoin
	if contract != "" {
		denom = string(contract)
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

func (txBuilder TxBuilder) calculateFees(amount xc.AmountBlockchain, contractMaybe xc.ContractAddress, input *tx_input.TxInput, includeTax bool) types.Coins {
	// amount := args.GetAmount()
	asset := txBuilder.Asset
	gasDenom := asset.GasCoin
	if gasDenom == "" {
		gasDenom = asset.ChainCoin
	}
	feeCoins := types.Coins{
		{
			Denom:  gasDenom,
			Amount: math.NewIntFromUint64(uint64(input.GasPrice * float64(input.GasLimit))),
		},
	}
	if includeTax {
		taxRate := txBuilder.Asset.ChainTransferTax
		tax := GetTaxFrom(amount, taxRate)
		if tax.Uint64() > 0 {
			taxDenom := asset.ChainCoin
			if contractMaybe != "" {
				taxDenom = string(contractMaybe)
			}
			taxStr, _ := math.NewIntFromString(tax.String())
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
	}
	// Must be sorted or cosmos client panics
	sort.Slice(feeCoins, func(i, j int) bool {
		return feeCoins[i].Denom < feeCoins[j].Denom
	})
	return feeCoins
}

type txArgs struct {
	Memo          string
	FromPublicKey []byte
}

// createTxWithMsg creates a new Tx given Cosmos Msg
func (txBuilder TxBuilder) createTxWithMsg(input *tx_input.TxInput, msg types.Msg, args txArgs, fees types.Coins) (xc.Tx, error) {
	return tx.NewTx(
		txBuilder.Asset,
		*input,
		[]types.Msg{msg},
		fees,
		args.FromPublicKey,
		args.Memo,
	), nil
}
