package ton

import (
	"crypto/ed25519"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	xc "github.com/cordialsys/crosschain"
	"github.com/cordialsys/crosschain/chain/ton/api"
	"github.com/sirupsen/logrus"
	"github.com/xssnick/tonutils-go/address"
	"github.com/xssnick/tonutils-go/tlb"
	"github.com/xssnick/tonutils-go/ton/wallet"
	"github.com/xssnick/tonutils-go/tvm/cell"
)

// TxBuilder for Template
type TxBuilder struct {
	Asset xc.ITask
}

var _ xc.TxBuilder = &TxBuilder{}

// NewTxBuilder creates a new Template TxBuilder
func NewTxBuilder(cfgI xc.ITask) (xc.TxBuilder, error) {
	return TxBuilder{
		Asset: cfgI,
	}, nil
}

// NewTransfer creates a new transfer for an Asset, either native or token
func (txBuilder TxBuilder) NewTransfer(from xc.Address, to xc.Address, amount xc.AmountBlockchain, input xc.TxInput) (xc.Tx, error) {
	if _, ok := txBuilder.Asset.(*xc.TokenAssetConfig); ok {
		return txBuilder.NewTokenTransfer(from, to, amount, input)
	}
	return txBuilder.NewNativeTransfer(from, to, amount, input)
}

// NewNativeTransfer creates a new transfer for a native asset
func (txBuilder TxBuilder) NewNativeTransfer(from xc.Address, to xc.Address, amount xc.AmountBlockchain, input xc.TxInput) (xc.Tx, error) {
	txInput := input.(*TxInput)

	var stateInit *tlb.StateInit
	var err error
	if txInput.AccountStatus != api.Active {
		if len(txInput.PublicKey) == 0 {
			return nil, fmt.Errorf("did not set public-key in tx-input for new ton account %s", from)
		}
		stateInit, err = wallet.GetStateInit(ed25519.PublicKey(txInput.PublicKey), DefaultWalletVersion, DefaultSubwalletId)
		if err != nil {
			return nil, err
		}
	}

	toAddr, err := ParseAddress(to)
	if err != nil {
		return nil, fmt.Errorf("invalid TON destination %s: %v", to, err)
	}
	fromAddr, err := ParseAddress(from)
	if err != nil {
		return nil, fmt.Errorf("invalid TON address %s: %v", to, err)
	}
	tfMsg, err := BuildTransfer(toAddr, tlb.FromNanoTON(amount.Int()), false, txInput.Memo)
	if err != nil {
		return nil, err
	}
	msgs := []*wallet.Message{}

	msgs = append(msgs, tfMsg)

	logrus.WithFields(logrus.Fields{
		"messages":   len(msgs),
		"state-init": stateInit != nil,
		"chain":      txBuilder.Asset.GetChain().Chain,
	}).Debug("building tx")

	cellBuilder, err := BuildV3UnsignedMessage(txInput, msgs)
	if err != nil {
		return nil, err
	}

	return NewTx(fromAddr, cellBuilder, stateInit), nil
}

// NewTokenTransfer creates a new transfer for a token asset
func (txBuilder TxBuilder) NewTokenTransfer(from xc.Address, to xc.Address, amount xc.AmountBlockchain, input xc.TxInput) (xc.Tx, error) {
	return nil, errors.New("not implemented")
}

func BuildTransfer(to *address.Address, amount tlb.Coins, bounce bool, comment string) (_ *wallet.Message, err error) {
	var body *cell.Cell
	if comment != "" {
		body, err = wallet.CreateCommentCell(comment)
		if err != nil {
			return nil, err
		}
	}

	return &wallet.Message{
		Mode: wallet.PayGasSeparately + wallet.IgnoreErrors,
		InternalMessage: &tlb.InternalMessage{
			IHRDisabled: true,
			Bounce:      bounce,
			DstAddr:     to,
			Amount:      amount,
			Body:        body,
		},
	}, nil
}

// func BuildAccountInit(from *address.Address, stateInit *tlb.StateInit) (_ *wallet.Message, err error) {
// 	return &wallet.Message{
// 		Mode: wallet.PayGasSeparately + wallet.IgnoreErrors,
// 		InternalMessage: &tlb.InternalMessage{
// 			IHRDisabled: false,
// 			Bounce:      false,
// 			DstAddr:     from,
// 			Amount:      tlb.Coins{},
// 			Body:        nil,
// 			StateInit:   stateInit,
// 		},
// 	}, nil
// }

func BuildV3UnsignedMessage(txInput *TxInput, messages []*wallet.Message) (*cell.Builder, error) {
	if len(messages) > 4 {
		return nil, errors.New("for this type of wallet max 4 messages can be sent in the same time")
	}

	seq := txInput.Sequence
	expiration := time.Unix(txInput.Timestamp, 0).Add(2 * time.Hour).Unix()
	payload := cell.BeginCell().MustStoreUInt(DefaultSubwalletId, 32).
		MustStoreUInt(uint64(expiration), 32).
		MustStoreUInt(uint64(seq), 32)

	for i, message := range messages {
		intMsg, err := tlb.ToCell(message.InternalMessage)
		if err != nil {
			return nil, fmt.Errorf("failed to convert internal message %d to cell: %w", i, err)
		}
		boc := intMsg.ToBOCWithFlags(false)
		fmt.Println("intmsg: ", hex.EncodeToString(boc))
		payload.MustStoreUInt(uint64(message.Mode), 8).MustStoreRef(intMsg)

	}

	// sign := payload.EndCell().Sign(s.wallet.key)
	// msg := cell.BeginCell().MustStoreSlice(sign, 512).MustStoreBuilder(payload).EndCell()

	return payload, nil
}
