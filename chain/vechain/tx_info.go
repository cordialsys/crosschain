package vechain

import (
	"context"
	"fmt"
	"math/big"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	xc "github.com/cordialsys/crosschain"
	clienterrors "github.com/cordialsys/crosschain/client/errors"
	txinfo "github.com/cordialsys/crosschain/client/tx_info"
	"github.com/ethereum/go-ethereum/common/hexutil"
)

type receipt struct {
	GasPayer xc.Address `json:"gasPayer"`
	Paid     string     `json:"paid"`
	Reverted bool       `json:"reverted"`
	Meta     struct {
		Origin         xc.Address `json:"txOrigin"`
		BlockID        string     `json:"blockID"`
		BlockNumber    uint64     `json:"blockNumber"`
		BlockTimestamp int64      `json:"blockTimestamp"`
	} `json:"meta"`
	Outputs []struct {
		Transfers []struct {
			Sender    xc.Address `json:"sender"`
			Recipient xc.Address `json:"recipient"`
			Amount    string     `json:"amount"`
		} `json:"transfers"`
		Events []struct {
			Address xc.ContractAddress `json:"address"`
			Topics  []string           `json:"topics"`
			Data    string             `json:"data"`
		} `json:"events"`
	} `json:"outputs"`
}

func (client *Client) FetchLegacyTxInfo(ctx context.Context, hash xc.TxHash) (txinfo.LegacyTxInfo, error) {
	info := txinfo.LegacyTxInfo{TxID: string(hash), FeeContract: VTHOContract}
	id, err := hexutil.Decode(string(hash))
	if err != nil || len(id) != 32 {
		return info, fmt.Errorf("invalid VeChain transaction ID")
	}
	var r *receipt
	if err := client.request(ctx, http.MethodGet, "/transactions/"+url.PathEscape(string(hash))+"/receipt", nil, &r); err != nil {
		return info, err
	}
	if r == nil {
		return info, clienterrors.Errorf(clienterrors.TransactionNotFound, "VeChain transaction %s has no receipt", hash)
	}
	info.Fee, err = parseAmount(r.Paid)
	if err != nil {
		return info, err
	}
	info.From = xc.Address(strings.ToLower(string(r.Meta.Origin)))
	info.FeePayer = xc.Address(strings.ToLower(string(r.GasPayer)))
	info.BlockHash = r.Meta.BlockID
	info.BlockIndex = int64(r.Meta.BlockNumber)
	info.BlockTime, info.Time = r.Meta.BlockTimestamp, r.Meta.BlockTimestamp
	head, err := client.getBlock(ctx, "best")
	if err != nil {
		return info, err
	}
	if head.Number >= r.Meta.BlockNumber {
		info.Confirmations = int64(head.Number - r.Meta.BlockNumber + 1)
	}
	if r.Reverted {
		info.Status, info.Error = xc.TxStatusFailure, "VeChain transaction reverted"
		return info, nil
	}
	appendMovement := func(from, to xc.Address, amount xc.AmountBlockchain, contract xc.ContractAddress, event *txinfo.Event) {
		info.Sources = append(info.Sources, &txinfo.LegacyTxInfoEndpoint{Address: xc.Address(strings.ToLower(string(from))), NativeAsset: client.Asset.Chain, Amount: amount, ContractAddress: xc.ContractAddress(strings.ToLower(string(contract))), Event: event})
		info.Destinations = append(info.Destinations, &txinfo.LegacyTxInfoEndpoint{Address: xc.Address(strings.ToLower(string(to))), NativeAsset: client.Asset.Chain, Amount: amount, ContractAddress: xc.ContractAddress(strings.ToLower(string(contract))), Event: event})
	}
	for i, output := range r.Outputs {
		for j, transfer := range output.Transfers {
			amount, err := parseAmount(transfer.Amount)
			if err != nil {
				return info, err
			}
			appendMovement(transfer.Sender, transfer.Recipient, amount, "", txinfo.NewEvent(fmt.Sprintf("%d/transfer/%d", i, j), txinfo.MovementVariantNative))
		}
		for j, event := range output.Events {
			// ERC-20/VIP-180 Transfer(address,address,uint256); exclude ERC-721's
			// four-topic event, which has the same event signature.
			if len(event.Topics) != 3 || !strings.EqualFold(event.Topics[0], "0xddf252ad1be2c89b69c2b068fc378daa952ba7f163c4a11628f55a4df523b3ef") {
				continue
			}
			from, e1 := hexutil.Decode(event.Topics[1])
			to, e2 := hexutil.Decode(event.Topics[2])
			data, e3 := hexutil.Decode(event.Data)
			if e1 != nil || e2 != nil || e3 != nil || len(from) != 32 || len(to) != 32 || len(data) != 32 {
				return info, fmt.Errorf("invalid Thor token transfer event")
			}
			amount := xc.AmountBlockchain(*new(big.Int).SetBytes(data))
			appendMovement(xc.Address(hexutil.Encode(from[12:])), xc.Address(hexutil.Encode(to[12:])), amount, event.Address, txinfo.NewEvent(strconv.Itoa(i)+"/event/"+strconv.Itoa(j), txinfo.MovementVariantToken))
		}
	}
	if len(info.Sources) > 0 {
		info.From, info.To = info.Sources[0].Address, info.Destinations[0].Address
		info.Amount, info.ContractAddress = info.Destinations[0].Amount, info.Destinations[0].ContractAddress
	}
	return info, nil
}

func (client *Client) FetchTxInfo(ctx context.Context, args *txinfo.Args) (txinfo.TxInfo, error) {
	legacy, err := client.FetchLegacyTxInfo(ctx, args.TxHash())
	if err != nil {
		return txinfo.TxInfo{}, err
	}
	return txinfo.TxInfoFromLegacy(client.Asset, legacy, txinfo.Account), nil
}

func CheckError(err error) clienterrors.Status {
	if err == nil {
		return clienterrors.UnknownError
	}
	message := strings.ToLower(err.Error())
	switch {
	case strings.Contains(message, "eof"), strings.Contains(message, "connection refused"), strings.Contains(message, "timeout"), strings.Contains(message, "thor http 5"):
		return clienterrors.NetworkError
	case strings.Contains(message, "insufficient energy"):
		return clienterrors.NoBalanceForGas
	case strings.Contains(message, "insufficient balance"):
		return clienterrors.NoBalance
	case strings.Contains(message, "already exists"), strings.Contains(message, "known tx"):
		return clienterrors.TransactionExists
	case strings.Contains(message, "expired"):
		return clienterrors.TransactionTimedOut
	case strings.Contains(message, "reverted"):
		return clienterrors.TransactionFailure
	}
	return clienterrors.UnknownError
}
