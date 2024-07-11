package bitcoin

import (
	"errors"
	"fmt"

	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcd/txscript"
	"github.com/btcsuite/btcd/wire"
	xc "github.com/cordialsys/crosschain"
	"github.com/cordialsys/crosschain/chain/bitcoin/address"
	"github.com/cordialsys/crosschain/chain/bitcoin/params"
	"github.com/cordialsys/crosschain/chain/bitcoin/tx"
	"github.com/cordialsys/crosschain/chain/bitcoin/tx_input"
	"github.com/sirupsen/logrus"
)

const TxVersion int32 = 2

// TxBuilder for Bitcoin
type TxBuilder struct {
	Asset          xc.ITask
	Params         *chaincfg.Params
	AddressDecoder address.AddressDecoder
	// isBch  bool
}

// NewTxBuilder creates a new Bitcoin TxBuilder
func NewTxBuilder(cfgI xc.ITask) (xc.TxBuilder, error) {
	native := cfgI.GetChain()
	params, err := params.GetParams(native)
	if err != nil {
		return TxBuilder{}, err
	}
	return TxBuilder{
		Asset:          cfgI,
		Params:         params,
		AddressDecoder: &address.BtcAddressDecoder{},
		// isBch:  native.Chain == xc.BCH,
	}, nil
}

func (txBuilder TxBuilder) WithAddressDecoder(decoder address.AddressDecoder) TxBuilder {
	txBuilder.AddressDecoder = decoder
	return txBuilder
}

// NewTransfer creates a new transfer for an Asset, either native or token
func (txBuilder TxBuilder) NewTransfer(from xc.Address, to xc.Address, amount xc.AmountBlockchain, input xc.TxInput) (xc.Tx, error) {
	switch asset := txBuilder.Asset.(type) {
	case *xc.ChainConfig:
		return txBuilder.NewNativeTransfer(from, to, amount, input)
	case *xc.TokenAssetConfig:
		return txBuilder.NewTokenTransfer(from, to, amount, input)
	default:
		return nil, fmt.Errorf("NewTransfer not implemented for %T", asset)
	}
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
	// 255 for bitcoin, 300 for bch
	estimatedTxBytesLength := xc.NewAmountBlockchainFromUint64(uint64(255 * len(local_input.UnspentOutputs)))
	if xc.NativeAsset(txBuilder.Asset.GetChain().Chain) == xc.BCH {
		estimatedTxBytesLength = xc.NewAmountBlockchainFromUint64(uint64(300 * len(local_input.UnspentOutputs)))
	}
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
				fee.ToHuman(txBuilder.Asset.GetDecimals()).String(), diff.ToHuman(txBuilder.Asset.GetDecimals()).String(),
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

// NewTokenTransfer creates a new transfer for a token asset
func (txBuilder TxBuilder) NewTokenTransfer(from xc.Address, to xc.Address, amount xc.AmountBlockchain, input xc.TxInput) (xc.Tx, error) {
	return nil, errors.New("not implemented")
}
