package builder

import (
	"fmt"

	xc "github.com/cordialsys/crosschain"
	xcbuilder "github.com/cordialsys/crosschain/builder"
	cantonaddress "github.com/cordialsys/crosschain/chain/canton/address"
	cantontx "github.com/cordialsys/crosschain/chain/canton/tx"
	"github.com/cordialsys/crosschain/chain/canton/tx_input"
)

// TxBuilder for Canton
type TxBuilder struct {
	Asset *xc.ChainBaseConfig
}

var _ xcbuilder.FullTransferBuilder = TxBuilder{}

// NewTxBuilder creates a new Canton TxBuilder
func NewTxBuilder(cfgI *xc.ChainBaseConfig) (TxBuilder, error) {
	return TxBuilder{
		Asset: cfgI,
	}, nil
}

// Transfer creates a new transfer for an Asset, either native or token
func (txBuilder TxBuilder) Transfer(args xcbuilder.TransferArgs, input xc.TxInput) (xc.Tx, error) {
	if contract, ok := args.GetContract(); ok {
		return txBuilder.NewTokenTransfer(args, contract, input)
	}
	return txBuilder.NewNativeTransfer(args, input)
}

// NewNativeTransfer creates a Tx from the prepared transaction in TxInput.
// The heavy lifting (command building, prepare call) was done in FetchTransferInput.
func (txBuilder TxBuilder) NewNativeTransfer(args xcbuilder.TransferArgs, input xc.TxInput) (xc.Tx, error) {
	cantonInput, ok := input.(*tx_input.TxInput)
	if !ok {
		return nil, fmt.Errorf("invalid tx input type for Canton, expected *tx_input.TxInput")
	}

	// Extract the key fingerprint from the sender's party ID
	// Party format: name::12<fingerprint>
	_, fingerprint, err := cantonaddress.ParsePartyID(args.GetFrom())
	if err != nil {
		return nil, fmt.Errorf("failed to parse sender party ID: %w", err)
	}

	tx := &cantontx.Tx{
		PreparedTransaction:     &cantonInput.PreparedTransaction,
		PreparedTransactionHash: cantonInput.Sighash,
		HashingSchemeVersion:    cantonInput.HashingSchemeVersion,
		Party:                   string(args.GetFrom()),
		KeyFingerprint:          fingerprint,
		SubmissionId:            cantonInput.SubmissionId,
	}

	return tx, nil
}

// NewTokenTransfer is not supported for Canton
func (txBuilder TxBuilder) NewTokenTransfer(args xcbuilder.TransferArgs, contract xc.ContractAddress, input xc.TxInput) (xc.Tx, error) {
	return nil, fmt.Errorf("token transfers are not supported for %s", txBuilder.Asset.Chain)
}
