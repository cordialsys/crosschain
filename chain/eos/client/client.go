package client

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	xc "github.com/cordialsys/crosschain"
	xcbuilder "github.com/cordialsys/crosschain/builder"
	txinfo "github.com/cordialsys/crosschain/client/tx_info"
	"github.com/sirupsen/logrus"

	"github.com/cordialsys/crosschain/chain/eos/builder"
	"github.com/cordialsys/crosschain/chain/eos/builder/action"
	eos "github.com/cordialsys/crosschain/chain/eos/eos-go"
	"github.com/cordialsys/crosschain/chain/eos/tx_input"
	xclient "github.com/cordialsys/crosschain/client"
	xctypes "github.com/cordialsys/crosschain/client/types"
)

// Client for Template
type Client struct {
	api   *eos.API
	chain *xc.ChainConfig
}

var _ xclient.Client = &Client{}
var _ xclient.StakingClient = &Client{}

func NewClient(cfgI *xc.ChainConfig) (*Client, error) {
	url := cfgI.GetChain().URL
	auth := cfgI.GetChain().Auth2
	apiKey := ""
	if auth != "" {
		var err error
		apiKey, err = auth.Load()
		if err != nil {
			return nil, fmt.Errorf("failed to load EOS client api key: %w", err)
		}
	}

	api2 := eos.New(url, cfgI.GetChain().DefaultHttpClient().Timeout)
	api2.Header.Set("Content-Type", "application/json")
	if apiKey != "" {
		api2.Header.Set("x-api-key", apiKey)
	}

	return &Client{api: api2, chain: cfgI.GetChain()}, nil
}

// FetchTransferInput returns tx input for a Template tx
func (client *Client) FetchTransferInput(ctx context.Context, args xcbuilder.TransferArgs) (xc.TxInput, error) {
	from := args.GetFrom()
	contract, _ := args.GetContract()
	account, _ := args.GetFromIdentity()
	feePayer, _ := args.GetFeePayer()
	feePayerIdentity, _ := args.GetFeePayerIdentity()
	return client.FetchBaseTxInput(
		ctx,
		AddressAndAccount{Address: from, Account: account},
		AddressAndAccount{Address: feePayer, Account: feePayerIdentity},
		contract,
	)
}

type AddressAndAccount struct {
	Address xc.Address
	Account string
}

func (client *Client) ResolveAccount(ctx context.Context, address xc.Address) (string, error) {
	accountsResp, err := client.api.GetAccountsByAuthorizers(ctx, []eos.PermissionLevel{}, []string{string(address)})
	if err != nil {
		return "", fmt.Errorf("failed to get accounts by authorizers: %v", err)
	}
	if len(accountsResp.Accounts) == 0 {
		return "", fmt.Errorf("no account found for '%s', you need to create an EOS account first", address)
	}
	accounts := map[string]bool{}
	for _, account := range accountsResp.Accounts {
		accounts[string(account.Account)] = true
	}
	if len(accounts) > 1 {
		return "", fmt.Errorf("multiple accounts found for '%s', but no identity set for the address", address)
	}
	return string(accountsResp.Accounts[0].Account), nil
}

func (client *Client) FetchBaseTxInput(ctx context.Context, from AddressAndAccount, feePayerMaybe AddressAndAccount, contractMaybe xc.ContractAddress) (*tx_input.TxInput, error) {
	info, err := client.api.GetInfo(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get EOS info: %w", err)
	}

	if from.Account == "" {
		from.Account, err = client.ResolveAccount(ctx, from.Address)
		if err != nil {
			return nil, err
		}
	}
	if feePayerMaybe.Address != "" && feePayerMaybe.Account == "" {
		feePayerMaybe.Account, err = client.ResolveAccount(ctx, feePayerMaybe.Address)
		if err != nil {
			return nil, err
		}
	}

	accountInfo, err := client.api.GetAccount(ctx, eos.AccountName(from.Account))
	if err != nil {
		return nil, fmt.Errorf("failed to get account info for '%s': %v", from.Account, err)
	}
	ramAvailable := accountInfo.RAMQuota - accountInfo.RAMUsage
	eosAmount := xc.NewAmountBlockchainFromUint64(uint64(accountInfo.CoreLiquidBalance.Amount))

	input := tx_input.NewTxInput()
	input.ChainID = info.ChainID
	input.HeadBlockID = info.HeadBlockID
	input.Timestamp = info.HeadBlockTime.Time.Unix()
	input.FromAccount = from.Account
	input.FeePayerAccount = feePayerMaybe.Account

	input.AvailableRam = int64(ramAvailable)
	input.AvailableCPU = int64(accountInfo.CPULimit.Available)
	input.AvailableNET = int64(accountInfo.NetLimit.Available)
	input.TargetRam = tx_input.TargetRam
	input.EosBalance = eosAmount

	contract := contractMaybe
	if contract == "" {
		contract = tx_input.DefaultContractId(client.chain.Base())
	}

	_, symbol, err := tx_input.ParseContractId(client.chain.Base(), contract, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to parse contract ID: %w", err)
	}
	input.Symbol = symbol

	return input, nil
}

func (client *Client) FetchLegacyTxInput(ctx context.Context, from xc.Address, to xc.Address) (xc.TxInput, error) {
	// No way to pass the amount in the input using legacy interface, so we estimate using min amount.
	chainCfg := client.chain.Base()
	args, _ := xcbuilder.NewTransferArgs(chainCfg, from, to, xc.NewAmountBlockchainFromUint64(1))
	return client.FetchTransferInput(ctx, args)
}

func (client *Client) SubmitTx(ctx context.Context, tx xctypes.SubmitTxReq) error {
	// This should be the JSON serialized transaction
	bz, err := tx.Serialize()
	if err != nil {
		return fmt.Errorf("failed to serialize EOS tx: %w", err)
	}
	_, err = client.api.PushRawTransaction(ctx, bz)
	if err != nil {
		return fmt.Errorf("failed to push EOS tx: %w", err)
	}

	return nil
}

// Returns transaction info - legacy/old endpoint
func (client *Client) FetchLegacyTxInfo(ctx context.Context, txHash xc.TxHash) (txinfo.LegacyTxInfo, error) {
	return txinfo.LegacyTxInfo{}, errors.New("not implemented")
}

// Returns transaction info - new endpoint
func (client *Client) FetchTxInfo(ctx context.Context, args *txinfo.Args) (txinfo.TxInfo, error) {
	txHash := args.TxHash()
	tx, err := client.api.GetTransactionFromAnySupportedEndpoint(ctx, string(txHash))
	if err != nil {
		return txinfo.TxInfo{}, fmt.Errorf("failed to get EOS tx: %w", err)
	}
	err = tx.Validate()
	if err != nil {
		return txinfo.TxInfo{}, fmt.Errorf("failed to validate EOS tx: %w", err)
	}

	native := client.chain.Chain

	chainInfo, err := client.api.GetInfo(ctx)
	if err != nil {
		return txinfo.TxInfo{}, fmt.Errorf("failed to get EOS chain info: %w", err)
	}
	// TODO is there an tx with an error?

	txInfo := txinfo.NewTxInfo(
		txinfo.NewBlock(native, tx.GetBlockNum(), tx.GetBlockId(), tx.GetBlockTime()),
		client.chain,
		tx.GetTxId(),
		uint64(chainInfo.HeadBlockNum-uint32(tx.GetBlockNum())),
		nil,
	)

	// There often seems to be redundent traces, so we need to dedup them.
	recordedActions := map[string]bool{}
	for _, trace := range tx.GetActions() {
		// skip traces with no receipt
		if !trace.Ok() {
			continue
		}
		actionId := trace.GetId()
		if recordedActions[actionId] {
			continue
		}
		recordedActions[actionId] = true

		switch trace.GetName() {
		case "transfer":
			data := action.Transfer{}
			err = json.Unmarshal(trace.GetData(), &data)
			if err != nil {
				return txinfo.TxInfo{}, err
			}

			contract := xc.ContractAddress(trace.GetAccount() + "/" + data.Quantity.Symbol.Symbol)
			if contract == "eosio.token/EOS" {
				contract = ""
			}
			movement := txinfo.NewMovement(native, contract)

			decimals := 4
			amount := xc.NewAmountBlockchainFromUint64(uint64(data.Quantity.Amount))
			movement.AddSource(xc.Address(data.From), amount, &decimals)
			movement.AddDestination(xc.Address(data.To), amount, &decimals)
			txInfo.AddMovement(movement)
		case "delegatebw":
			data := action.DelegateBWOutputOnly{}
			err = json.Unmarshal(trace.GetData(), &data)
			if err != nil {
				return txinfo.TxInfo{}, err
			}

			var rawAddress string
			accountInfo, err := client.api.GetAccount(ctx, eos.AccountName(data.From))
			if err != nil {
				logrus.WithFields(logrus.Fields{
					"account": data.From,
				}).Error("failed to get EOS account info")
			} else {
				rawAddress = accountInfo.GetRawAddressFromPermissions()
			}

			staking := txinfo.Stake{
				Validator: "",
				Balance:   data.CPUQuantity.ToBlockchain(action.Decimals),
				Account:   string(data.From),
				Address:   rawAddress,
			}

			if !data.CPUQuantity.IsZero() {
				cpuStaking := staking
				cpuStaking.Validator = string(builder.CPU)
				txInfo.Stakes = append(txInfo.Stakes, &cpuStaking)
			}
			if !data.NetQuantity.IsZero() {
				netStaking := staking
				netStaking.Validator = string(builder.NET)
				txInfo.Stakes = append(txInfo.Stakes, &netStaking)
			}
		case "undelegatebw":
			data := action.UnDelegateBWOutputOnly{}
			err = json.Unmarshal(trace.GetData(), &data)
			if err != nil {
				return txinfo.TxInfo{}, err
			}

			var rawAddress string
			accountInfo, err := client.api.GetAccount(ctx, eos.AccountName(data.From))
			if err != nil {
				logrus.WithFields(logrus.Fields{
					"account": data.From,
				}).Error("failed to get EOS account info")
			} else {
				rawAddress = accountInfo.GetRawAddressFromPermissions()
			}

			unstaking := txinfo.Unstake{
				Balance:   data.CPUQuantity.ToBlockchain(action.Decimals),
				Account:   string(data.From),
				Address:   rawAddress,
				Validator: string(builder.CPU),
			}

			if !data.CPUQuantity.IsZero() {
				cpuUnstaking := unstaking
				cpuUnstaking.Validator = string(builder.CPU)
				txInfo.Unstakes = append(txInfo.Unstakes, &cpuUnstaking)
			}
			if !data.NetQuantity.IsZero() {
				netUnstaking := unstaking
				netUnstaking.Validator = string(builder.NET)
				txInfo.Unstakes = append(txInfo.Unstakes, &netUnstaking)
			}
		}
	}
	return *txInfo, nil
}

func (client *Client) FetchBalance(ctx context.Context, args *xclient.BalanceArgs) (xc.AmountBlockchain, error) {
	from := args.Address()
	accounts, err := client.api.GetAccountsByAuthorizers(ctx, []eos.PermissionLevel{}, []string{string(from)})
	if err != nil {
		return xc.AmountBlockchain{}, fmt.Errorf("failed to get EOS accounts: %w", err)
	}

	contract, ok := args.Contract()
	if !ok {
		contract = tx_input.DefaultContractId(client.chain.Base())
	}

	var contractAccount, symbol string
	contractAccount, symbol, err = tx_input.ParseContractId(client.chain.Base(), contract, nil)
	if err != nil {
		return xc.AmountBlockchain{}, err
	}

	total := xc.AmountBlockchain{}
	accountsLookedAt := map[eos.AccountName]bool{}
	for _, account := range accounts.Accounts {
		// There can be multiple entries for the same account, one for each permission.
		if accountsLookedAt[account.Account] {
			continue
		}
		assets, err := client.api.GetCurrencyBalance(ctx, account.Account, symbol, eos.AccountName(contractAccount))
		if err != nil {
			return xc.AmountBlockchain{}, fmt.Errorf("failed to get '%s' balance: %w", contract, err)
		}
		if len(assets) > 0 {
			bal := xc.NewAmountBlockchainFromUint64(uint64(assets[0].Amount))
			total = total.Add(&bal)
		}
		accountsLookedAt[account.Account] = true
	}
	return total, nil
}

func (client *Client) FetchDecimals(ctx context.Context, contract xc.ContractAddress) (int, error) {
	// seems it's always 4 decimals for EOS assets
	return 4, nil
}

func (client *Client) FetchBlock(ctx context.Context, args *xclient.BlockArgs) (*txinfo.BlockWithTransactions, error) {
	height, ok := args.Height()
	if !ok {
		info, err := client.api.GetInfo(ctx)
		if err != nil {
			return nil, err
		}
		height = uint64(info.HeadBlockNum)
	}

	resp, err := client.api.GetBlockByNum(ctx, uint32(height))
	if err != nil {
		return nil, err
	}
	native := client.chain.Chain
	transactions := []string{}
	for _, tx := range resp.Transactions {
		transactions = append(transactions, tx.Transaction.ID.String())
	}

	block := txinfo.NewBlock(native, height, resp.ID.String(), resp.Timestamp.Time)
	return &txinfo.BlockWithTransactions{Block: *block, TransactionIds: transactions}, nil
}
