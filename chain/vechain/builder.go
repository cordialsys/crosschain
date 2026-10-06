package vechain

import (
	"fmt"
	"strings"

	xc "github.com/cordialsys/crosschain"
	xcbuilder "github.com/cordialsys/crosschain/builder"
	"github.com/cordialsys/crosschain/chain/evm"
	evmtx "github.com/cordialsys/crosschain/chain/evm/tx"
	"github.com/ethereum/go-ethereum/common/hexutil"
)

type TxBuilder struct{ Asset *xc.ChainBaseConfig }

func NewTxBuilder(cfg *xc.ChainBaseConfig) (TxBuilder, error) { return TxBuilder{Asset: cfg}, nil }
func (TxBuilder) SupportsMemo() xc.MemoSupport                { return xc.MemoSupportNone }

func transferClause(cfg *xc.ChainBaseConfig, args xcbuilder.TransferArgs) (clause, error) {
	if err := evm.ValidateAddress(cfg, args.GetFrom()); err != nil {
		return clause{}, err
	}
	if err := evm.ValidateAddress(cfg, args.GetTo()); err != nil {
		return clause{}, err
	}
	if args.GetAmount().Int().Sign() < 0 || args.GetAmount().Int().BitLen() > 256 {
		return clause{}, fmt.Errorf("VeChain amount must fit uint256")
	}
	if _, ok := args.GetFeePayer(); ok {
		return clause{}, fmt.Errorf("VeChain fee delegation is not supported")
	}
	if fee, ok := args.GetFeeContract(); ok && !strings.EqualFold(string(fee), string(VTHOContract)) {
		return clause{}, fmt.Errorf("VeChain fees must be paid in VTHO")
	}
	if contract, ok := args.GetContract(); ok {
		if err := evm.ValidateAddress(cfg, xc.Address(contract)); err != nil {
			return clause{}, err
		}
	}
	to, value, data, err := evmtx.EvmDestinationAndAmountAndData(args.GetTo(), args.GetAmount(), &args)
	return clause{To: to, Value: value, Data: data}, err
}

func (builder TxBuilder) Transfer(args xcbuilder.TransferArgs, raw xc.TxInput) (xc.Tx, error) {
	input, ok := raw.(*TxInput)
	if !ok || input == nil {
		return nil, fmt.Errorf("expected VeChain transaction input, got %T", raw)
	}
	if input.FeePayerAddress != "" {
		return nil, fmt.Errorf("VeChain fee delegation is not supported")
	}
	if configured := string(builder.Asset.ChainID); configured != "" {
		genesis, err := hexutil.Decode(configured)
		if err != nil || len(genesis) != 32 || genesis[31] != input.ChainTag {
			return nil, fmt.Errorf("VeChain input chain tag does not match configured network")
		}
	}
	c, err := transferClause(builder.Asset, args)
	if err != nil {
		return nil, err
	}
	if input.FromAddress != "" && !strings.EqualFold(string(input.FromAddress), string(args.GetFrom())) {
		return nil, fmt.Errorf("VeChain sender differs from transaction input")
	}
	if input.Expiration == 0 || input.GasLimit < intrinsicGas(c) {
		return nil, fmt.Errorf("invalid VeChain expiration or gas limit")
	}
	if input.GasFeeCap.Int().Sign() <= 0 || input.GasTipCap.Int().Sign() < 0 || input.GasTipCap.Cmp(&input.GasFeeCap) > 0 {
		return nil, fmt.Errorf("invalid VeChain gas fee caps")
	}
	return &Tx{sender: args.GetFrom(), envelope: envelope{
		ChainTag: input.ChainTag, BlockRef: input.BlockRef, Expiration: input.Expiration,
		Clauses: []clause{c}, GasTipCap: input.GasTipCap.Int(), GasFeeCap: input.GasFeeCap.Int(),
		Gas: input.GasLimit, Nonce: input.Nonce,
	}}, nil
}

// Thor charges 5,000 gas per transaction and 16,000 per non-creation clause.
func intrinsicGas(c clause) uint64 {
	gas := uint64(21_000)
	for _, b := range c.Data {
		if b == 0 {
			gas += 4
		} else {
			gas += 68
		}
	}
	return gas
}
