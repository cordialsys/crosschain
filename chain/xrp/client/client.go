package client

import (
	"context"
	"encoding/hex"
	"fmt"
	"net/http"
	"time"

	xc "github.com/cordialsys/crosschain"
	xcbuilder "github.com/cordialsys/crosschain/builder"
	"github.com/cordialsys/crosschain/chain/xrp/address/contract"
	"github.com/cordialsys/crosschain/chain/xrp/client/events"
	"github.com/cordialsys/crosschain/chain/xrp/client/types"
	xrptxinput "github.com/cordialsys/crosschain/chain/xrp/tx_input"
	xclient "github.com/cordialsys/crosschain/client"
	"github.com/cordialsys/crosschain/client/errors"
	txinfo "github.com/cordialsys/crosschain/client/tx_info"
	xctypes "github.com/cordialsys/crosschain/client/types"
	"github.com/sirupsen/logrus"
)

// Client for XRP
type Client struct {
	Url        string
	HttpClient *http.Client
	Asset      xc.ITask
}

var _ xclient.Client = &Client{}

// NewClient returns a new JSON-RPC Client to the XRP node
func NewClient(cfgI xc.ITask) (*Client, error) {
	cfg := cfgI.GetChain()

	return &Client{
		Url:        cfg.URL,
		HttpClient: cfg.DefaultHttpClient(),
		Asset:      cfgI,
	}, nil
}

const MethodPost string = "POST"

func (client *Client) FetchTransferInput(ctx context.Context, args xcbuilder.TransferArgs) (xc.TxInput, error) {
	txInput := xrptxinput.NewTxInput()

	account := args.GetFrom()

	accountInfo, err := client.getAccountInfo(account)
	if err != nil {
		return nil, err
	}
	currentSequencePtr := accountInfo.Result.AccountData.Sequence
	txInput.V2Sequence = currentSequencePtr

	txInput.XrpBalance = xc.NewAmountBlockchainFromStr(accountInfo.Result.AccountData.Balance)
	// Currently the reserve amount is 1XRP and the delete-account fee is 0.2XRP
	// We'll use the 0.2 as the threshold for account deletion.
	txInput.AccountDeleteFee = xc.NewAmountBlockchainFromUint64(200_000)
	txInput.ReserveAmount = xc.NewAmountBlockchainFromUint64(200_000)
	reserveAmountHuman := client.Asset.GetChain().ChainClientConfig.ReserveAmount
	if !reserveAmountHuman.IsZero() {
		reserveAmount := reserveAmountHuman.ToBlockchain(client.Asset.GetChain().GetDecimals())
		txInput.ReserveAmount = reserveAmount
	}

	tfAmount := args.GetAmount()
	remainder := txInput.XrpBalance.Sub(&tfAmount)
	zero := xc.NewAmountBlockchainFromUint64(0)
	if remainder.Cmp(&zero) < 0 {
		decimals := client.Asset.GetChain().GetDecimals()
		return nil, fmt.Errorf(
			"insufficient balance: %s would remain after transfering %s",
			remainder.ToHuman(decimals),
			tfAmount.ToHuman(decimals),
		)
	}
	if remainder.Cmp(&txInput.ReserveAmount) <= 0 {
		logrus.WithFields(logrus.Fields{
			"balance": txInput.XrpBalance,
			"reserve": txInput.ReserveAmount,
			"amount":  tfAmount,
		}).Debug("XRP balance is less than reserve amount, setting account delete")
		txInput.AccountDelete = true
	}

	ledger, err := client.getLatestLedger(false)
	if err != nil {
		return nil, err
	}
	ledgerSequencePtr := ledger.Result.LedgerCurrentIndex
	ledgerOffset := int64(20) // Ledger offset
	lastLedgerSequence := ledgerSequencePtr + ledgerOffset
	txInput.V2LastLedgerSequence = lastLedgerSequence

	feeInfo, err := client.getFee()
	if err != nil {
		return nil, err
	}

	// XRP has very confusing method of going about prioritization.
	// But fee itself is at least a simple fixed fee.
	// Current approach:
	// - Use the median fee, based on recent ledger
	// - Use the minimum base fee if it's greater than the median fee, as a sanity check
	txInput.Fee = feeInfo.Result.Drops.MedianFee
	if feeInfo.Result.Drops.BaseFee.Cmp(&txInput.Fee) > 0 {
		// Somehow the median is less than the base fee -> use the base fee
		txInput.Fee = feeInfo.Result.Drops.BaseFee
	}

	return txInput, nil
}

// Deprecated method - use FetchTransferInput
func (client *Client) FetchLegacyTxInput(ctx context.Context, from xc.Address, to xc.Address) (xc.TxInput, error) {
	// No way to pass the amount in the input using legacy interface, so we estimate using min amount.
	chainCfg := client.Asset.GetChain().Base()
	args, _ := xcbuilder.NewTransferArgs(chainCfg, from, to, xc.NewAmountBlockchainFromUint64(1))
	txInput, err := client.FetchTransferInput(ctx, args)
	if err != nil {
		return nil, err
	}

	return txInput, nil
}

// SubmitTx submits a Template tx
func (client *Client) SubmitTx(ctx context.Context, txInput xctypes.SubmitTxReq) error {
	serializedTxInputBytes, err := txInput.Serialize()
	if err != nil {
		return err
	}

	serializedTxInputHex := hex.EncodeToString(serializedTxInputBytes)
	serializedTxInputHexBytes := []byte(serializedTxInputHex)

	submitResponse, err := client.postSubmit(serializedTxInputHexBytes)
	if err != nil {
		return err
	}

	if submitResponse.Result.EngineResultCode != 0 {
		return fmt.Errorf(
			"transaction not accepted: %s (%s)",
			submitResponse.Result.EngineResultMessage,
			submitResponse.Result.EngineResult,
		)
	}

	return nil
}

// FetchTxInfo Returns transaction info - new endpoint
func (client *Client) FetchTxInfo(ctx context.Context, args *txinfo.Args) (txinfo.TxInfo, error) {
	txHash := args.TxHash()
	txInfo, err := client.GetTxInfo(ctx, txHash)
	if err != nil {
		return txinfo.TxInfo{}, err
	}

	return txInfo, nil
}

func (client *Client) FetchLegacyTxInfo(ctx context.Context, txHash xc.TxHash) (txinfo.LegacyTxInfo, error) {
	return txinfo.LegacyTxInfo{}, fmt.Errorf("unimplemented")
}

func (client *Client) GetTxInfo(ctx context.Context, txHash xc.TxHash) (txinfo.TxInfo, error) {
	txRequest := &types.TransactionRequest{
		Method: "tx",
		Params: []types.TransactionParamEntry{
			{
				Transaction: txHash,
				Binary:      false,
			},
		},
	}

	var txResponse types.TransactionResponse
	err := client.Send(MethodPost, txRequest, &txResponse)
	if err != nil {
		return txinfo.TxInfo{}, err
	}
	if txResponse.Result.Hash == "" {
		return txinfo.TxInfo{}, errors.TransactionNotFoundf("no transaction by hash '%s'", txHash)
	}

	ledger, err := client.getLatestLedger(false)
	if err != nil {
		return txinfo.TxInfo{}, err
	}
	chainCfg := client.Asset.GetChain()
	chain := chainCfg.Chain

	blockTime := time.Unix(types.XRP_EPOCH+txResponse.Result.Date, 0)

	block := txinfo.NewBlock(chain, uint64(txResponse.Result.LedgerIndex), "", blockTime)

	confirmations := ledger.Result.LedgerCurrentIndex - txResponse.Result.LedgerIndex

	var errMsg *string
	if txResponse.Result.Meta.TransactionResult != "tesSUCCESS" {
		msg := fmt.Sprintf("transaction failed: %s", txResponse.Result.Meta.TransactionResult)
		errMsg = &msg
	}

	txInfo := txinfo.NewTxInfo(block, chainCfg, txResponse.Result.Hash, uint64(confirmations), errMsg)

	affectedNodes := txResponse.Result.Meta.AffectedNodes

	for _, node := range affectedNodes {
		xrpNode, ok, err := events.NewEvent(node)
		if !ok {
			// skip
			continue
		}
		if err != nil {
			return txinfo.TxInfo{}, err
		}

		// Fetch address, contract and amount
		address, err := xrpNode.GetAddress(&txResponse)
		if err != nil {
			return txinfo.TxInfo{}, err
		}

		contract, err := xrpNode.GetContract()
		if err != nil {
			return txinfo.TxInfo{}, err
		}

		amount, err := xrpNode.GetAmount()
		if err != nil {
			return txinfo.TxInfo{}, err
		}
		// XRP sometimes reports balances as negative
		amount = amount.Abs()

		movement := txinfo.NewMovement(chainCfg.Chain, contract)
		isSource, err := xrpNode.IsSource(&txResponse)
		if err != nil {
			return txinfo.TxInfo{}, err
		}
		if txResponse.Result.DestinationTag != 0 {
			movement.Memo = fmt.Sprintf("%d", txResponse.Result.DestinationTag)
		}

		if isSource {
			movement.AddSource(
				address,
				amount,
				nil,
			)
		} else {
			movement.AddDestination(
				address,
				amount,
				nil,
			)
		}
		txInfo.AddMovement(movement)
	}
	// We coalesce since the 'events' from XRP do not include both sender and recipient.
	// So the raw transfers we added aren't very clear, and we can simplify by merging together
	// based on asset.
	txInfo.Coalesece()

	txInfo.Fees = txInfo.CalculateFees()

	return *txInfo, nil
}

// FetchBalance fetches token balance for a XRP address
func (client *Client) FetchBalance(ctx context.Context, args *xclient.BalanceArgs) (xc.AmountBlockchain, error) {
	if contract, ok := args.Contract(); ok {
		return client.fetchContractBalance(ctx, args.Address(), contract)
	} else {
		return client.FetchNativeBalance(ctx, args.Address())
	}
}

// FetchNativeBalance fetches account native balance for a XRP address
func (client *Client) FetchNativeBalance(ctx context.Context, address xc.Address) (xc.AmountBlockchain, error) {
	zero := xc.NewAmountBlockchainFromUint64(0)
	accountInfoResponse, err := client.getAccountInfo(address)
	if err != nil {
		return zero, err
	}

	balance := accountInfoResponse.Result.AccountData.Balance
	if balance == "" {
		return zero, fmt.Errorf("empty balance returned for account: %s", address)
	}

	return xc.NewAmountBlockchainFromStr(balance), nil
}

// fetchContractBalance fetches a specific token balance based on received contract for an XRP address
func (client *Client) fetchContractBalance(ctx context.Context, address xc.Address, assetContract xc.ContractAddress) (xc.AmountBlockchain, error) {
	zero := xc.NewAmountBlockchainFromUint64(0)

	asset, contract, err := contract.ExtractAssetAndContract(assetContract)
	if err != nil {
		return zero, fmt.Errorf("failed to parse and extract asset and contract: %w", err)
	}

	accountLinesResponse, err := client.getAccountLines(address)
	if err != nil {
		return zero, err
	}

	var balance string
	for _, line := range accountLinesResponse.Result.Lines {
		if line.Currency == asset && line.Account == contract {
			balance = line.Balance
		}
	}

	if balance == "" {
		return zero, fmt.Errorf("empty balance returned for account: %s", address)
	}

	humanReadbleBalance, err := xc.NewAmountHumanReadableFromStr(balance)
	if err != nil {
		return zero, fmt.Errorf("failed to parse balance for account: %s", address)
	}
	return humanReadbleBalance.ToBlockchain(types.TRUSTLINE_DECIMALS), nil
}

// Pretty simple for XRP as it's always fixed.
func (client *Client) FetchDecimals(ctx context.Context, contract xc.ContractAddress) (int, error) {
	if client.Asset.GetChain().IsChain(contract) {
		return types.XRP_NATIVE_DECIMALS, nil
	}

	return types.TRUSTLINE_DECIMALS, nil
}

func (client *Client) FetchBlock(ctx context.Context, args *xclient.BlockArgs) (*txinfo.BlockWithTransactions, error) {
	var ledger *types.LedgerResponse
	var err error
	var ledgerHash string
	height, ok := args.Height()
	if !ok {
		ledger, err = client.getLatestLedger(true)
		if err != nil {
			return nil, err
		}
		// unable to get ledgerData on head of chain
	} else {
		ledger, err = client.getLedger(types.LedgerIndex(fmt.Sprint(height)), true)
		if err != nil {
			return nil, err
		}
		// fetch data to get ledger hash
		data, err := client.getLedgerData(types.LedgerIndex(ledger.Result.Ledger.LedgerIndex))
		if err != nil {
			return nil, err
		}
		ledgerHash = data.Result.LedgerHash
	}

	block := &txinfo.BlockWithTransactions{
		Block: *txinfo.NewBlock(
			client.Asset.GetChain().Chain,
			xc.NewAmountBlockchainFromStr(ledger.Result.Ledger.LedgerIndex).Uint64(),
			ledgerHash,
			time.Unix(types.XRP_EPOCH+ledger.Result.Ledger.CloseTime, 0),
		),
		TransactionIds: ledger.Result.Ledger.Transactions,
	}

	return block, nil

}
