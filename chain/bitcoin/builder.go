package bitcoin

import (
	"errors"
	"fmt"

	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcd/txscript"
	"github.com/btcsuite/btcd/wire"
	xc "github.com/cordialsys/crosschain"
	xcbuilder "github.com/cordialsys/crosschain/builder"
	"github.com/cordialsys/crosschain/chain/bitcoin/address"
	"github.com/cordialsys/crosschain/chain/bitcoin/params"
	"github.com/cordialsys/crosschain/chain/bitcoin/tx"
	"github.com/cordialsys/crosschain/chain/bitcoin/tx_input"
	"github.com/sirupsen/logrus"
)

const TxVersion int32 = 2

// TxBuilder for Bitcoin
type TxBuilder struct {
	Asset          *xc.ChainBaseConfig
	Params         *chaincfg.Params
	AddressDecoder address.AddressDecoder
	// isBch  bool
}

var _ xcbuilder.FullTransferBuilder = &TxBuilder{}

// NewTxBuilder creates a new Bitcoin TxBuilder
func NewTxBuilder(cfgI *xc.ChainBaseConfig) (TxBuilder, error) {
	params, err := params.GetParams(cfgI)
	if err != nil {
		return TxBuilder{}, err
	}
	return TxBuilder{
		Asset:          cfgI,
		Params:         params,
		AddressDecoder: &address.BtcAddressDecoder{},
	}, nil
}

func (txBuilder TxBuilder) WithAddressDecoder(decoder address.AddressDecoder) TxBuilder {
	txBuilder.AddressDecoder = decoder
	return txBuilder
}

// NewTransfer creates a new transfer for an Asset, either native or token
func (txBuilder TxBuilder) Transfer(args xcbuilder.TransferArgs, input xc.TxInput) (xc.Tx, error) {
	if _, ok := args.GetContract(); ok {
		return nil, fmt.Errorf("token transfers are not supported on %s", txBuilder.Asset.Chain)
	}

	return txBuilder.NewNativeTransfer(args.GetFrom(), args.GetTo(), args.GetAmount(), input)
}

// NewNativeTransfer creates a new transfer for a native asset
func (txBuilder TxBuilder) NewNativeTransfer(from xc.Address, to xc.Address, amount xc.AmountBlockchain, input xc.TxInput) (xc.Tx, error) {
	var local_input *tx_input.TxInput
	var ok bool
	if local_input, ok = (input.(*tx_input.TxInput)); !ok {
		return &tx.Tx{}, errors.New("xc.TxInput is not from a bitcoin chain")
	}
	// Only need to save min utxo for the transfer.
	totalSpend := local_input.SumUtxo()

	gasPrice := local_input.GasPricePerByte
	estimatedTxBytesLength := xc.NewAmountBlockchainFromUint64(
		local_input.GetEstimatedSizePerSpentUtxo() * uint64(len(local_input.UnspentOutputs)),
	)
	fee := gasPrice.Mul(&estimatedTxBytesLength)

	transferAmountAndFee := amount.Add(&fee)
	unspentAmountMinusTransferAndFee := totalSpend.Sub(&transferAmountAndFee)
	recipients := []tx.Recipient{
		{
			To:    to,
			Value: amount,
		},
		{
			To:    from,
			Value: unspentAmountMinusTransferAndFee,
		},
	}

	msgTx := wire.NewMsgTx(TxVersion)

	for _, input := range local_input.UnspentOutputs {
		hash := chainhash.Hash{}
		copy(hash[:], input.Hash)
		msgTx.AddTxIn(wire.NewTxIn(wire.NewOutPoint(&hash, input.Index), nil, nil))
	}

	// Outputs
	for _, recipient := range recipients {
		addr, err := txBuilder.AddressDecoder.Decode(recipient.To, txBuilder.Params)
		if err != nil {
			return nil, err
		}
		script, err := txscript.PayToAddrScript(addr)
		if err != nil {
			logrus.WithError(err).WithField("to", recipient.To).Error("trying paytoaddr")
			return nil, err
		}
		value := recipient.Value.Int().Int64()
		if value < 0 {
			diff := local_input.SumUtxo().Sub(&amount)
			return nil, fmt.Errorf("not enough funds for fees, estimated fee is %s but only %s is left after transfer",
				fee.ToHuman(txBuilder.Asset.Decimals).String(), diff.ToHuman(txBuilder.Asset.Decimals).String(),
			)
		}
		msgTx.AddTxOut(wire.NewTxOut(value, script))
	}

	tx := tx.Tx{
		MsgTx: msgTx,

		From:   from,
		To:     to,
		Amount: amount,
		Input:  local_input,

		Recipients: recipients,
	}
	return &tx, nil
}
