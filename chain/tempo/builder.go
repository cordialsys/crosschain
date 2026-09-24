package tempo

import (
	"fmt"
	"strings"

	xc "github.com/cordialsys/crosschain"
	xcbuilder "github.com/cordialsys/crosschain/builder"
	evmbuilder "github.com/cordialsys/crosschain/chain/evm/builder"
	evmtx "github.com/cordialsys/crosschain/chain/evm/tx"
	evminput "github.com/cordialsys/crosschain/chain/evm/tx_input"
	"github.com/ethereum/go-ethereum/common"
)

type TxBuilder struct {
	evmbuilder.TxBuilder
}

func NewTxBuilder(cfg *xc.ChainBaseConfig) (TxBuilder, error) {
	evmBuilder, err := evmbuilder.NewTxBuilder(cfg)
	if err != nil {
		return TxBuilder{}, err
	}

	return TxBuilder{
		TxBuilder: evmBuilder,
	}, nil
}

var _ xcbuilder.FullBuilder = &TxBuilder{}

func (txBuilder TxBuilder) Transfer(args xcbuilder.TransferArgs, input xc.TxInput) (xc.Tx, error) {
	if err := validateTransfer(args); err != nil {
		return nil, err
	}
	evmInput, err := evmTransferInput(input)
	if err != nil {
		return nil, err
	}
	feeContract, explicit := args.GetFeeContract()
	feePayer, sponsored := args.GetFeePayer()
	if tempoInput, ok := input.(*TxInput); ok {
		if explicit && !strings.EqualFold(string(feeContract), string(tempoInput.FeeContract)) {
			return nil, fmt.Errorf("fee contract differs from the token used for fee estimation")
		}
		if !explicit {
			feeContract = tempoInput.FeeContract
		}
		if tempoInput.NativeFeePayer != sponsored || !strings.EqualFold(string(tempoInput.FeePayerAddress), string(feePayer)) || tempoInput.FeePayerNonce != 0 || tempoInput.BasicSmartAccountNonce != 0 {
			return nil, fmt.Errorf("Tempo fee payer differs from native sponsorship input: requested payer=%q, input payer=%q, native_fee_payer=%t (expected %t), fee_payer_nonce=%d, basic_smart_account_nonce=%d; fetch fresh input with the same Tempo client version and fee-payer arguments",
				feePayer, tempoInput.FeePayerAddress, tempoInput.NativeFeePayer, sponsored, tempoInput.FeePayerNonce, tempoInput.BasicSmartAccountNonce)
		}
	} else if sponsored || evmInput.FeePayerAddress != "" {
		return nil, fmt.Errorf("native sponsorship requires a Tempo transaction input")
	}
	if evmInput.FromAddress != "" && !strings.EqualFold(string(evmInput.FromAddress), string(args.GetFrom())) {
		return nil, fmt.Errorf("Tempo sender differs from transaction input")
	}
	// Explicitly build a single EVM call: NewTx would select EIP-7702 for a payer.
	ethTx, err := evmtx.NewSingleTx(args, evmInput, txBuilder.Asset).BuildEthTx()
	if err != nil {
		return nil, err
	}
	return newTempoTx(args, ethTx, feeContract)
}

func validateTransfer(args xcbuilder.TransferArgs) error {
	contract, ok := args.GetContract()
	if !ok || !common.IsHexAddress(string(contract)) {
		return fmt.Errorf("Tempo requires a valid transfer token contract")
	}
	if contract, ok := args.GetFeeContract(); ok && (!common.IsHexAddress(string(contract)) || common.HexToAddress(string(contract)) == (common.Address{})) {
		return fmt.Errorf("Tempo requires a valid fee token contract")
	}
	if !common.IsHexAddress(string(args.GetFrom())) || !common.IsHexAddress(string(args.GetTo())) {
		return fmt.Errorf("Tempo requires valid sender and recipient addresses")
	}
	if payer, ok := args.GetFeePayer(); ok && (!common.IsHexAddress(string(payer)) || common.HexToAddress(string(payer)) == (common.Address{})) {
		return fmt.Errorf("Tempo requires a valid fee-payer address")
	}
	if args.GetAmount().Int().Sign() < 0 || args.GetAmount().Int().BitLen() > 256 {
		return fmt.Errorf("Tempo transfer amount must fit uint256")
	}
	return nil
}

func (txBuilder TxBuilder) MultiTransfer(args xcbuilder.MultiTransferArgs, input xc.MultiTransferInput) (xc.Tx, error) {
	switch input := input.(type) {
	case *MultiTransferInput:
		return evmtx.NewMultiTx(txBuilder.Asset, args, &input.TxInput.TxInput)
	case *evminput.MultiTransferInput:
		return evmtx.NewMultiTx(txBuilder.Asset, args, &input.TxInput)
	default:
		return nil, fmt.Errorf("unsupported Tempo multi-transfer input type %T", input)
	}
}

func evmTransferInput(input xc.TxInput) (*evminput.TxInput, error) {
	switch input := input.(type) {
	case *TxInput:
		if input == nil {
			return nil, fmt.Errorf("Tempo transaction input is nil")
		}
		return &input.TxInput, nil
	case *evminput.TxInput:
		if input == nil {
			return nil, fmt.Errorf("EVM transaction input is nil")
		}
		return input, nil
	default:
		return nil, fmt.Errorf("unsupported Tempo transfer input type %T", input)
	}
}
