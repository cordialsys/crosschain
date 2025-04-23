package ton

import (
	"crypto/ed25519"
	"errors"
	"fmt"
	"math/big"
	"time"

	xc "github.com/cordialsys/crosschain"
	xcbuilder "github.com/cordialsys/crosschain/builder"
	tonaddress "github.com/cordialsys/crosschain/chain/ton/address"
	"github.com/cordialsys/crosschain/chain/ton/api"
	tontx "github.com/cordialsys/crosschain/chain/ton/tx"
	"github.com/sirupsen/logrus"
	"github.com/xssnick/tonutils-go/address"
	"github.com/xssnick/tonutils-go/tlb"
	"github.com/xssnick/tonutils-go/ton/jetton"
	"github.com/xssnick/tonutils-go/ton/wallet"
	"github.com/xssnick/tonutils-go/tvm/cell"
)

var Zero = xc.NewAmountBlockchainFromUint64(0)

// TxBuilder for Template
type TxBuilder struct {
	Asset *xc.ChainBaseConfig
}

var _ xcbuilder.FullTransferBuilder = &TxBuilder{}

// NewTxBuilder creates a new Template TxBuilder
func NewTxBuilder(cfgI *xc.ChainBaseConfig) (TxBuilder, error) {
	return TxBuilder{
		Asset: cfgI,
	}, nil
}

// NewTransfer creates a new transfer for an Asset, either native or token
func (txBuilder TxBuilder) Transfer(args xcbuilder.TransferArgs, input xc.TxInput) (xc.Tx, error) {
	from := args.GetFrom()
	to := args.GetTo()
	amount := args.GetAmount()

	txInput := input.(*TxInput)

	var stateInit *tlb.StateInit
	var err error
	if txInput.AccountStatus != api.Active {
		fromPubKey, ok := args.GetPublicKey()
		if !ok || len(fromPubKey) == 0 {
			return nil, fmt.Errorf("must set from-public-key in transfer args for new ton account: %s", from)
		}
		stateInit, err = wallet.GetStateInit(ed25519.PublicKey(fromPubKey), tonaddress.DefaultWalletVersion, tonaddress.DefaultSubwalletId)
		if err != nil {
			return nil, err
		}
	}
	net := txBuilder.Asset.Network

	toAddr, err := tonaddress.ParseAddress(to, net)
	if err != nil {
		return nil, fmt.Errorf("invalid TON destination %s: %v", to, err)
	}
	fromAddr, err := tonaddress.ParseAddress(from, net)
	if err != nil {
		return nil, fmt.Errorf("invalid TON address %s: %v", to, err)
	}

	amountTlb, err := tlb.FromNano((*big.Int)(&amount), int(txBuilder.Asset.Decimals))
	if err != nil {
		return nil, err
	}
	memo, _ := args.GetMemo()

	msgs := []*wallet.Message{}
	// Token transfer
	if contract, ok := args.GetContract(); ok {
		tokenAddr, err := tonaddress.ParseAddress(txInput.TokenWallet, net)
		if err != nil {
			return nil, fmt.Errorf("invalid TON token address %s: %v", txInput.TokenWallet, err)
		}
		// Derive the token wallet address to protect from spending the wrong token
		walletAddresses, err := tonaddress.CalculatePossibleTokenWalletAddresses(from, contract, txInput.JettonWalletCode)
		if err != nil {
			return nil, fmt.Errorf("could not calcuate jetton wallet address for %s: %v", contract, err)
		}

		matches := false
		for _, possibleTokenAddr := range walletAddresses {
			if xc.Address(tokenAddr.String()) == possibleTokenAddr {
				matches = true
			}
		}
		if !matches {
			return nil, fmt.Errorf("could not validate token wallet address %s", txInput.TokenWallet)
		}

		// Spend max 0.2 TON per Jetton transfer.  If we don't have 0.2 TON, we should
		// lower the max to our balance less max-fees.
		maxJettonFee := xc.NewAmountBlockchainFromUint64(200000000)
		remainingTonBal := txInput.TonBalance.Sub(&txInput.EstimatedMaxFee)
		if maxJettonFee.Cmp(&remainingTonBal) > 0 && remainingTonBal.Cmp(&Zero) > 0 {
			maxJettonFee = remainingTonBal
		}
		// If the estimated max fee is greater, then we should use that.
		if txInput.EstimatedMaxFee.Cmp(&maxJettonFee) > 0 {
			maxJettonFee = txInput.EstimatedMaxFee
		}

		tfMsg, err := BuildJettonTransfer(uint64(txInput.Timestamp), fromAddr, tokenAddr, toAddr, amountTlb, tlb.FromNanoTON(maxJettonFee.Int()), memo)
		if err != nil {
			return nil, err
		}
		msgs = append(msgs, tfMsg)

	} else {
		// Native transfer
		tfMsg, err := BuildTransfer(toAddr, amountTlb, false, memo)
		if err != nil {
			return nil, err
		}
		msgs = append(msgs, tfMsg)
	}

	logrus.WithFields(logrus.Fields{
		"messages":   len(msgs),
		"state-init": stateInit != nil,
		"chain":      txBuilder.Asset.Chain,
	}).Debug("building tx")

	cellBuilder, err := BuildV3UnsignedMessage(txInput, msgs)
	if err != nil {
		return nil, err
	}

	return tontx.NewTx(fromAddr, cellBuilder, stateInit), nil
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

// Jetton is the TON token standard
func BuildJettonTransfer(randomInt uint64, from *address.Address, tokenWallet *address.Address, to *address.Address, amount tlb.Coins, maxFee tlb.Coins, comment string) (_ *wallet.Message, err error) {
	var body *cell.Cell
	if comment != "" {
		body, err = wallet.CreateCommentCell(comment)
		if err != nil {
			return nil, err
		}
	}

	tokenBody, err := tlb.ToCell(jetton.TransferPayload{
		QueryID:     randomInt,
		Amount:      amount,
		Destination: to,
		// For some reason we have to send TON with the jetton transfer
		// and this address receives excess TON that wasn't needed.
		ResponseDestination: from,
		CustomPayload:       body,
		ForwardTONAmount:    tlb.ZeroCoins,
		ForwardPayload:      nil,
	})
	if err != nil {
		return nil, err
	}
	return wallet.SimpleMessage(tokenWallet, maxFee, tokenBody), nil
}

func BuildV3UnsignedMessage(txInput *TxInput, messages []*wallet.Message) (*cell.Builder, error) {
	// TON v3 wallets have a max of 4 messages
	if len(messages) > 4 {
		return nil, errors.New("for this type of wallet max 4 messages can be sent in the same time")
	}

	seq := txInput.Sequence
	expiration := time.Unix(txInput.Timestamp, 0).Add(2 * time.Hour).Unix()
	payload := cell.BeginCell().MustStoreUInt(tonaddress.DefaultSubwalletId, 32).
		MustStoreUInt(uint64(expiration), 32).
		MustStoreUInt(uint64(seq), 32)

	for i, message := range messages {
		intMsg, err := tlb.ToCell(message.InternalMessage)
		if err != nil {
			return nil, fmt.Errorf("failed to convert internal message %d to cell: %w", i, err)
		}
		payload.MustStoreUInt(uint64(message.Mode), 8).MustStoreRef(intMsg)
	}

	return payload, nil
}

func ParseComment(body *cell.Cell) (string, bool) {
	if body != nil {
		l := body.BeginParse()
		if val, err := l.LoadUInt(32); err == nil && val == 0 {
			str, _ := l.LoadStringSnake()
			return str, true
		}
	}
	return "", false
}
