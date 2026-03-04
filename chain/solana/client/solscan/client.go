package solscan

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	xc "github.com/cordialsys/crosschain"
	"github.com/sirupsen/logrus"

	"github.com/cordialsys/crosschain/client/errors"
	txinfo "github.com/cordialsys/crosschain/client/tx_info"
)

type Client struct {
	baseURL *url.URL
	httpCli *http.Client
}

func NewClient(rawURL string) (*Client, error) {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return nil, fmt.Errorf("invalid indexer_url: %w", err)
	}
	return &Client{baseURL: parsed, httpCli: http.DefaultClient}, nil
}

func (c *Client) DoGet(ctx context.Context, u *url.URL, output any) (int, error) {
	log := logrus.WithFields(logrus.Fields{
		"url": u.String(),
	})
	log.Debug("get solscan")

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return 0, fmt.Errorf("failed to create indexer request: %w", err)
	}
	resp, err := c.httpCli.Do(req)
	if err != nil {
		return 0, fmt.Errorf("failed to fetch tx from indexer: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return resp.StatusCode, fmt.Errorf("failed to read indexer response body: %w", err)
	}

	log = log.WithFields(logrus.Fields{
		"data": string(body),
	})

	log.Debug("get solscan response")

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return resp.StatusCode, fmt.Errorf("indexer status %d", resp.StatusCode)
	}

	if err := json.Unmarshal(body, &output); err != nil {
		return resp.StatusCode, fmt.Errorf("failed to decode indexer response: %w", err)
	}
	return resp.StatusCode, nil
}

func (c *Client) GetTransactionDetail(ctx context.Context, txHash string) (*TxDetailResponse, error) {
	u := c.baseURL.JoinPath("v1/transaction/detail")
	q := u.Query()
	q.Set("tx", txHash)
	u.RawQuery = q.Encode()

	var parsed TxDetailResponse
	status, err := c.DoGet(ctx, u, &parsed)
	if err != nil {
		if status == http.StatusNotFound {
			return nil, errors.TransactionNotFoundf("not found")
		}
		return nil, err
	}

	if !parsed.Success || parsed.Data == nil {
		return nil, errors.TransactionNotFoundf("not found")
	}
	return &parsed, nil
}

// This is not reliable:
func (c *Client) GetLatestBlock(ctx context.Context) (*BlockData, error) {
	u := c.baseURL.JoinPath("v1/block/last")
	q := u.Query()
	q.Set("limit", "1")
	u.RawQuery = q.Encode()

	var parsed BlockResponse
	_, err := c.DoGet(ctx, u, &parsed)
	if err != nil {
		return nil, fmt.Errorf("latest block fetch failed: %w", err)
	}

	if !parsed.Success || len(parsed.Data) == 0 {
		return nil, fmt.Errorf("latest block not found")
	}
	return &parsed.Data[0], nil
}

func (c *Client) GetChainInfo(ctx context.Context) (*ChainInfoData, error) {
	u := c.baseURL.JoinPath("v1/common/chaininfo")

	var parsed ChainInfoResponse
	_, err := c.DoGet(ctx, u, &parsed)
	if err != nil {
		return nil, fmt.Errorf("latest block fetch failed: %w", err)
	}

	if !parsed.Success || parsed.Data == nil {
		return nil, fmt.Errorf("latest block not found")
	}
	return parsed.Data, nil
}

func (c *Client) GetLegacyTxInfo(ctx context.Context, txHash string) (txinfo.LegacyTxInfo, error) {
	result := txinfo.LegacyTxInfo{}

	parsed, err := c.GetTransactionDetail(ctx, string(txHash))
	if err != nil {
		return txinfo.LegacyTxInfo{}, err
	}

	data := parsed.Data
	result.TxID = data.TxID
	if result.TxID == "" {
		result.TxID = string(txHash)
	}
	result.BlockIndex = data.BlockID
	result.BlockTime = data.BlockTime
	result.Fee = xc.NewAmountBlockchainFromUint64(data.Fee)

	signers := data.Signer
	if len(signers) == 0 {
		signers = data.ListSigner
	}
	if len(signers) > 0 {
		result.FeePayer = xc.Address(signers[0])
	}

	seenEvents := map[string]struct{}{}
	for _, instruction := range data.ParsedInstructions {
		appendIndexerInstructionMovements(&result, instruction, seenEvents)
	}

	if len(result.Sources) > 0 {
		result.From = result.Sources[0].Address
	}
	if len(result.Destinations) > 0 {
		result.To = result.Destinations[0].Address
		result.Amount = result.Destinations[0].Amount
		result.ContractAddress = result.Destinations[0].ContractAddress
	}

	// For some reason solscan reports SOL as `So11111111111111111111111111111111111111111`,
	// So we need to swap it out.
	const solscanSolID = "So11111111111111111111111111111111111111111"
	for _, src := range result.Sources {
		if src.ContractAddress == xc.ContractAddress(solscanSolID) {
			src.ContractAddress = ""
		}
		if src.Asset == solscanSolID {
			src.Asset = ""
		}
	}
	for _, dest := range result.Destinations {
		if dest.ContractAddress == xc.ContractAddress(solscanSolID) {
			dest.ContractAddress = ""
		}
		if dest.Asset == solscanSolID {
			dest.Asset = ""
		}
	}

	if data.Status != 1 {
		result.Error = "transaction failed"
	}
	result.Confirmations = 0
	// Not ideal, but approximate and fine for a fallback method.
	if parsed.Data.TxStatus == "finalized" {
		// currently the final confirmations for solana is 150, so we should guess over that.
		result.Confirmations = 500
	}

	chainInfo, err := c.GetChainInfo(ctx)
	if err != nil {
		// return without chain-info...
		return result, nil
	}

	// The chainInfo endpoint can get pretty stale, but will provide some accuracy for older tx.
	if chainInfo.GetBlockHeight() > (parsed.Data.BlockID + 10) {
		result.Confirmations = chainInfo.GetBlockHeight() - parsed.Data.BlockID
	}

	return result, nil
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func parseIntentTransferInstruction(instruction Instruction) (string, string, uint64, bool) {
	if len(instruction.DataRaw) == 0 {
		return "", "", 0, false
	}
	if strings.EqualFold(string(instruction.DataRaw), "null") {
		return "", "", 0, false
	}
	// Some instructions return data_raw as a string; only parse object payloads.
	if instruction.DataRaw[0] != '{' {
		return "", "", 0, false
	}
	var dataRaw InstructionData
	if err := json.Unmarshal(instruction.DataRaw, &dataRaw); err != nil {
		return "", "", 0, false
	}
	if dataRaw.Info == nil {
		return "", "", 0, false
	}
	if !strings.EqualFold(instruction.Type, "intentTransfer") && !strings.EqualFold(dataRaw.Type, "intentTransfer") {
		return "", "", 0, false
	}
	amount, ok := parseSolscanAmount(dataRaw.Info.Lamports)
	if !ok || amount == 0 {
		return "", "", 0, false
	}
	if dataRaw.Info.Source == "" || dataRaw.Info.Destination == "" {
		return "", "", 0, false
	}
	return dataRaw.Info.Source, dataRaw.Info.Destination, amount, true
}

func eventID(index int, outerIndex *int) string {
	if outerIndex != nil && *outerIndex >= 0 {
		// inner instruction
		return fmt.Sprintf("%d.%d", *outerIndex+1, index+1)
	} else {
		// normal instruction
		return fmt.Sprintf("%d", index+1)
	}
}

func appendIndexerInstructionMovements(result *txinfo.LegacyTxInfo, instruction Instruction, seenEvents map[string]struct{}) {
	if len(instruction.Transfers) > 0 {
		transfer := instruction.Transfers[0]
		eventID := eventID(transfer.InsIndex, transfer.OuterInsIndex)
		from := firstNonEmpty(
			transfer.FromOwner,
			transfer.SourceOwner,
			transfer.FromAddress,
			transfer.SourceAddress,
		)
		to := firstNonEmpty(
			transfer.ToOwner,
			transfer.DestinationOwner,
			transfer.ToAddress,
			transfer.DestinationAddress,
		)
		if from != "" && to != "" {
			if amount, ok := solscanTransferAmount(transfer); ok && amount > 0 {
				if _, seen := seenEvents[eventID]; !seen {
					contract := firstNonEmpty(
						transfer.TokenContractAddress,
						transfer.TokenAddress,
						transfer.Mint,
					)
					var variant txinfo.MovementVariant = txinfo.MovementVariantNative
					contractAddress := xc.ContractAddress("")
					if contract != "" {
						variant = txinfo.MovementVariantToken
						contractAddress = xc.ContractAddress(contract)
					}
					event := txinfo.NewEvent(eventID, variant)
					xcAmount := xc.NewAmountBlockchainFromUint64(amount)
					result.Sources = append(result.Sources, &txinfo.LegacyTxInfoEndpoint{
						Address:         xc.Address(from),
						ContractAddress: contractAddress,
						Amount:          xcAmount,
						Event:           event,
					})
					result.Destinations = append(result.Destinations, &txinfo.LegacyTxInfoEndpoint{
						Address:         xc.Address(to),
						ContractAddress: contractAddress,
						Amount:          xcAmount,
						Event:           event,
					})
					seenEvents[eventID] = struct{}{}
				}
			}
		}
	} else {
		// fallback to parse some instructions that seem unsupported by solscan
		eventID := eventID(instruction.InsIndex, instruction.OuterInsIndex)
		source, destination, amount, ok := parseIntentTransferInstruction(instruction)
		if ok {
			if _, seen := seenEvents[eventID]; !seen {
				event := txinfo.NewEvent(eventID, txinfo.MovementVariantNative)
				xcAmount := xc.NewAmountBlockchainFromUint64(amount)
				result.Sources = append(result.Sources, &txinfo.LegacyTxInfoEndpoint{
					Address: xc.Address(source),
					Amount:  xcAmount,
					Event:   event,
				})
				result.Destinations = append(result.Destinations, &txinfo.LegacyTxInfoEndpoint{
					Address: xc.Address(destination),
					Amount:  xcAmount,
					Event:   event,
				})
				seenEvents[eventID] = struct{}{}
			}
		}
	}

	for _, innerInstruction := range instruction.InnerInstructions {
		appendIndexerInstructionMovements(result, innerInstruction, seenEvents)
	}
}

func solscanTransferAmount(transfer Transfer) (uint64, bool) {
	candidates := []interface{}{
		transfer.Amount,
		transfer.TokenAmount,
		transfer.Value,
		transfer.Lamports,
	}
	for _, candidate := range candidates {
		if amount, ok := parseSolscanAmount(candidate); ok {
			return amount, true
		}
	}
	return 0, false
}

func parseSolscanAmount(value interface{}) (uint64, bool) {
	switch typed := value.(type) {
	case nil:
		return 0, false
	case float64:
		if typed < 0 {
			return 0, false
		}
		return uint64(typed), true
	case json.Number:
		parsed, err := strconv.ParseUint(typed.String(), 10, 64)
		if err != nil {
			return 0, false
		}
		return parsed, true
	case string:
		parsed, err := strconv.ParseUint(typed, 10, 64)
		if err != nil {
			return 0, false
		}
		return parsed, true
	default:
		return 0, false
	}
}
