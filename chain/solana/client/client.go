package client

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"sort"
	"time"

	"github.com/cordialsys/crosschain/client/errors"

	xc "github.com/cordialsys/crosschain"
	"github.com/sirupsen/logrus"

	xcbuilder "github.com/cordialsys/crosschain/builder"
	"github.com/cordialsys/crosschain/chain/solana/builder"
	"github.com/cordialsys/crosschain/chain/solana/tx"
	"github.com/cordialsys/crosschain/chain/solana/tx_input"
	"github.com/cordialsys/crosschain/chain/solana/types"
	xclient "github.com/cordialsys/crosschain/client"
	bin "github.com/gagliardetto/binary"
	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/rpc"
)

// Client for Solana
type Client struct {
	SolClient *rpc.Client
	Asset     xc.ITask
}

var _ xclient.FullClient = &Client{}
var _ xclient.StakingClient = &Client{}
var _ xclient.ClientWithDecimals = &Client{}

// NewClient returns a new JSON-RPC Client to the Solana node
func NewClient(cfgI xc.ITask) (*Client, error) {
	cfg := cfgI.GetChain()
	solClient := rpc.New(cfg.URL)
	return &Client{
		SolClient: solClient,
		Asset:     cfgI,
	}, nil
}

func (client *Client) FetchBaseInput(ctx context.Context, fromAddr xc.Address) (*tx_input.TxInput, error) {
	txInput := tx_input.NewTxInput()

	// get recent block hash (i.e. nonce)
	recent, err := client.SolClient.GetLatestBlockhash(ctx, rpc.CommitmentFinalized)
	if err != nil {
		return nil, fmt.Errorf("could not get latest blockhash: %v", err)
	}
	if recent == nil || recent.Value == nil {
		return nil, fmt.Errorf("error fetching latest blockhash")
	}
	txInput.RecentBlockHash = recent.Value.Blockhash

	return txInput, nil
}

// FetchLegacyTxInput returns tx input for a Solana tx, namely a RecentBlockHash
func (client *Client) FetchTransferInput(ctx context.Context, args xcbuilder.TransferArgs) (xc.TxInput, error) {
	txInput, err := client.FetchBaseInput(ctx, args.GetFrom())
	if err != nil {
		return nil, err
	}

	asset := client.Asset
	contract := asset.GetContract()
	if contract == "" {
		if _, ok := asset.(*xc.ChainConfig); !ok {
			logrus.WithFields(logrus.Fields{
				"chain":      asset.GetChain().Chain,
				"asset_type": fmt.Sprintf("%T", asset),
			}).Warn("no associated contract but not native asset")
		}
		// native transfer
		return txInput, nil
	}

	mint, err := solana.PublicKeyFromBase58(contract)
	if err != nil {
		return nil, fmt.Errorf("invalid mint address: %s: %v", contract, err)
	}

	// determine token program for the token
	mintInfo, err := client.SolClient.GetAccountInfo(ctx, mint)
	if err != nil {
		return nil, err
	}
	txInput.TokenProgram = mintInfo.Value.Owner

	// get account info - check if to is an owner or ata
	accountTo, err := solana.PublicKeyFromBase58(string(args.GetTo()))
	if err != nil {
		return nil, err
	}

	// Determine if destination is a token account or not by
	// trying to lookup a token balance
	_, err = client.SolClient.GetTokenAccountBalance(ctx, accountTo, rpc.CommitmentFinalized)
	if err != nil {
		txInput.ToIsATA = false
	} else {
		txInput.ToIsATA = true
	}

	// for tokens, get ata account info
	ataTo := accountTo
	if !txInput.ToIsATA {
		ataToStr, err := types.FindAssociatedTokenAddress(string(args.GetTo()), contract, mintInfo.Value.Owner)
		if err != nil {
			return nil, err
		}
		ataTo = solana.MustPublicKeyFromBase58(ataToStr)
	}
	_, err = client.SolClient.GetAccountInfo(ctx, ataTo)
	if err != nil {
		// if the ATA doesn't exist yet, we will create when sending tokens
		txInput.ShouldCreateATA = true
	}

	// Fetch all token accounts as if they are utxo
	if contract != "" {
		tokenAccounts, err := client.GetTokenAccountsByOwner(ctx, string(args.GetFrom()), contract)
		if err != nil {
			return nil, err
		}
		zero := xc.NewAmountBlockchainFromUint64(0)
		for _, acc := range tokenAccounts {
			amount := xc.NewAmountBlockchainFromStr(acc.Info.Parsed.Info.TokenAmount.Amount)
			if amount.Cmp(&zero) > 0 {
				txInput.SourceTokenAccounts = append(txInput.SourceTokenAccounts, &tx_input.TokenAccount{
					Account: acc.Account.Pubkey,
					Balance: amount,
				})
			}
		}

		// To prevent dust issues, we sort descending and limit number of token accounts
		sort.Slice(txInput.SourceTokenAccounts, func(i, j int) bool {
			return txInput.SourceTokenAccounts[i].Balance.Cmp(&txInput.SourceTokenAccounts[j].Balance) > 0
		})
		if len(txInput.SourceTokenAccounts) > builder.MaxTokenTransfers {
			txInput.SourceTokenAccounts = txInput.SourceTokenAccounts[:builder.MaxTokenTransfers]
		}

		if len(tokenAccounts) == 0 {
			// no balance
			return nil, fmt.Errorf("no balance to send solana token")
		}
	}

	// fetch priority fee info
	accountsToLock := solana.PublicKeySlice{}
	accountsToLock = append(accountsToLock, mint)
	fees, err := client.SolClient.GetRecentPrioritizationFees(ctx, accountsToLock)
	if err != nil {
		return txInput, fmt.Errorf("could not lookup priority fees: %v", err)
	}
	priority_fee_count := uint64(0)
	// start with 100 min priority fee, then average in the recent priority fees paid.
	priority_fee_sum := uint64(100)
	for _, fee := range fees {
		if fee.PrioritizationFee > 0 {
			priority_fee_sum += fee.PrioritizationFee
			priority_fee_count += 1
		}
	}
	if priority_fee_count > 0 {
		txInput.PrioritizationFee = xc.NewAmountBlockchainFromUint64(
			priority_fee_sum / priority_fee_count,
		)
	} else {
		// default 100
		txInput.PrioritizationFee = xc.NewAmountBlockchainFromUint64(
			100,
		)
	}
	// apply multiplier
	txInput.PrioritizationFee = txInput.PrioritizationFee.ApplyGasPriceMultiplier(client.Asset.GetChain())

	return txInput, nil
}

func (client *Client) FetchLegacyTxInput(ctx context.Context, from xc.Address, to xc.Address) (xc.TxInput, error) {
	// No way to pass the amount in the input using legacy interface, so we estimate using min amount.
	args, _ := xcbuilder.NewTransferArgs(from, to, xc.NewAmountBlockchainFromUint64(1))
	return client.FetchTransferInput(ctx, args)
}

func (client *Client) SubmitTx(ctx context.Context, txInput xc.Tx) error {
	txData, err := txInput.Serialize()
	if err != nil {
		return fmt.Errorf("send transaction: encode transaction: %w", err)
	}

	_, err = client.SolClient.SendEncodedTransactionWithOpts(
		ctx,
		base64.StdEncoding.EncodeToString(txData),
		rpc.TransactionOpts{
			SkipPreflight:       false,
			PreflightCommitment: rpc.CommitmentFinalized,
		},
	)
	return err
}

// FetchLegacyTxInfo returns tx info for a Solana tx
func (client *Client) FetchLegacyTxInfo(ctx context.Context, txHash xc.TxHash) (xc.LegacyTxInfo, error) {
	result := xc.LegacyTxInfo{}

	txSig, err := solana.SignatureFromBase58(string(txHash))
	if err != nil {
		return result, err
	}
	// confusingly, '0' is the latest version, which comes after 'legacy' (no version).
	maxVersion := uint64(0)
	res, err := client.SolClient.GetTransaction(
		ctx,
		txSig,
		&rpc.GetTransactionOpts{
			Encoding:                       solana.EncodingBase64,
			Commitment:                     rpc.CommitmentFinalized,
			MaxSupportedTransactionVersion: &maxVersion,
		},
	)
	if err != nil {
		if err.Error() == "not found" {
			// similar to EVM, solana uses simple "not found" string
			return result, errors.TransactionNotFoundf("%v", err)
		}
		return result, err
	}
	if res == nil || res.Transaction == nil {
		return result, fmt.Errorf("invalid transaction in response")
	}

	solTx, err := solana.TransactionFromDecoder(bin.NewBinDecoder(res.Transaction.GetBinary()))
	if err != nil {
		return result, err
	}
	tx := tx.NewTxFrom(solTx)
	meta := res.Meta
	if res.BlockTime != nil {
		result.BlockTime = res.BlockTime.Time().Unix()
	}

	if res.Slot > 0 {
		result.BlockIndex = int64(res.Slot)
		if res.BlockTime != nil {
			result.BlockTime = int64(*res.BlockTime)
		}

		recent, err := client.SolClient.GetLatestBlockhash(ctx, rpc.CommitmentFinalized)
		if err != nil {
			// ignore
			logrus.WithError(err).Warn("failed to get latest blockhash")
		} else {
			result.Confirmations = int64(recent.Context.Slot) - result.BlockIndex
		}
	}
	result.Fee = xc.NewAmountBlockchainFromUint64(meta.Fee)

	result.TxID = string(txHash)

	sources := []*xc.LegacyTxInfoEndpoint{}
	dests := []*xc.LegacyTxInfoEndpoint{}

	for _, instr := range tx.GetSystemTransfers() {
		from := instr.GetFundingAccount().PublicKey.String()
		to := instr.GetRecipientAccount().PublicKey.String()
		amount := xc.NewAmountBlockchainFromUint64(*instr.Lamports)
		sources = append(sources, &xc.LegacyTxInfoEndpoint{
			Address: xc.Address(from),
			Amount:  amount,
		})
		dests = append(dests, &xc.LegacyTxInfoEndpoint{
			Address: xc.Address(to),
			Amount:  amount,
		})
	}
	for _, instr := range tx.GetVoteWithdraws() {
		from := instr.GetWithdrawAuthorityAccount().PublicKey.String()
		to := instr.GetRecipientAccount().PublicKey.String()
		amount := xc.NewAmountBlockchainFromUint64(*instr.Lamports)
		sources = append(sources, &xc.LegacyTxInfoEndpoint{
			Address: xc.Address(from),
			Amount:  amount,
		})
		dests = append(dests, &xc.LegacyTxInfoEndpoint{
			Address: xc.Address(to),
			Amount:  amount,
		})
	}
	for _, instr := range tx.GetStakeWithdraws() {
		from := instr.GetStakeAccount().PublicKey.String()
		to := instr.GetRecipientAccount().PublicKey.String()
		amount := xc.NewAmountBlockchainFromUint64(*instr.Lamports)
		sources = append(sources, &xc.LegacyTxInfoEndpoint{
			Address: xc.Address(from),
			Amount:  amount,
		})
		dests = append(dests, &xc.LegacyTxInfoEndpoint{
			Address: xc.Address(to),
			Amount:  amount,
		})
	}
	for _, instr := range tx.GetTokenTransferCheckeds() {
		from := instr.GetOwnerAccount().PublicKey.String()
		toTokenAccount := instr.GetDestinationAccount().PublicKey
		contract := xc.ContractAddress(instr.GetMintAccount().PublicKey.String())
		to := xc.Address(toTokenAccount.String())
		// Solana doesn't keep full historical state, so we can't rely on always being able to lookup the account.
		tokenAccountInfo, err := client.LookupTokenAccount(ctx, toTokenAccount)
		if err != nil {
			logrus.WithError(err).Warn("failed to lookup token account")
		} else {
			to = xc.Address(tokenAccountInfo.Parsed.Info.Owner)
		}

		amount := xc.NewAmountBlockchainFromUint64(*instr.Amount)
		sources = append(sources, &xc.LegacyTxInfoEndpoint{
			Address:         xc.Address(from),
			Amount:          amount,
			ContractAddress: contract,
		})
		dests = append(dests, &xc.LegacyTxInfoEndpoint{
			Address:         xc.Address(to),
			Amount:          amount,
			ContractAddress: contract,
		})
	}
	for _, instr := range tx.GetTokenTransfers() {
		from := instr.GetOwnerAccount().PublicKey.String()
		toTokenAccount := instr.GetDestinationAccount().PublicKey
		tokenAccountInfo, err := client.LookupTokenAccount(ctx, toTokenAccount)

		to := xc.Address(toTokenAccount.String())
		contract := xc.ContractAddress("")
		// Solana doesn't keep full historical state, so we can't rely on always being able to lookup the account.
		if err != nil {
			logrus.WithError(err).Warn("failed to lookup token account")
		} else {
			to = xc.Address(tokenAccountInfo.Parsed.Info.Owner)
			contract = xc.ContractAddress(tokenAccountInfo.Parsed.Info.Mint)
		}

		amount := xc.NewAmountBlockchainFromUint64(*instr.Amount)
		sources = append(sources, &xc.LegacyTxInfoEndpoint{
			Address:         xc.Address(from),
			Amount:          amount,
			ContractAddress: contract,
		})
		dests = append(dests, &xc.LegacyTxInfoEndpoint{
			Address:         xc.Address(to),
			Amount:          amount,
			ContractAddress: contract,
		})
	}
	for _, instr := range tx.GetDelegateStake() {
		xcStake := &xclient.Stake{
			Account:   instr.GetStakeAccount().PublicKey.String(),
			Validator: instr.GetVoteAccount().PublicKey.String(),
			Address:   instr.GetStakeAuthority().PublicKey.String(),
			// Needs to be looked up from separate instruction
			Balance: xc.AmountBlockchain{},
		}
		for _, createAccount := range tx.GetCreateAccounts() {
			if createAccount.NewAccount.Equals(instr.GetStakeAccount().PublicKey) {
				xcStake.Balance = xc.NewAmountBlockchainFromUint64(createAccount.Lamports)
			}
		}

		result.AddStakeEvent(xcStake)
	}
	for _, instr := range tx.GetDeactivateStakes() {
		xcStake := &xclient.Unstake{
			Account: instr.GetStakeAccount().PublicKey.String(),
			Address: instr.GetStakeAuthority().PublicKey.String(),

			// Needs to be looked up
			Balance:   xc.AmountBlockchain{},
			Validator: "",
		}
		stakeAccountInfo, err := client.LookupStakeAccount(ctx, instr.GetStakeAccount().PublicKey)
		if err != nil {
			logrus.WithError(err).Warn("failed to lookup stake account")
		} else {
			xcStake.Validator = stakeAccountInfo.Parsed.Info.Stake.Delegation.Voter
			xcStake.Balance = xc.NewAmountBlockchainFromStr(stakeAccountInfo.Parsed.Info.Stake.Delegation.Stake)
		}
		result.AddStakeEvent(xcStake)
	}

	if len(sources) > 0 {
		result.From = sources[0].Address
	}
	if len(dests) > 0 {
		result.To = dests[0].Address
		result.Amount = dests[0].Amount
		result.ContractAddress = dests[0].ContractAddress
	}

	if meta.Err != nil {
		// no movements
		errBz, _ := json.Marshal(meta.Err)
		result.Error = string(errBz)
	} else {
		result.Sources = sources
		result.Destinations = dests
	}

	return result, nil
}

func (client *Client) FetchTxInfo(ctx context.Context, txHashStr xc.TxHash) (xclient.TxInfo, error) {
	legacyTx, err := client.FetchLegacyTxInfo(ctx, txHashStr)
	if err != nil {
		return xclient.TxInfo{}, err
	}

	// remap to new tx
	return xclient.TxInfoFromLegacy(client.Asset.GetChain(), legacyTx, xclient.Account), nil
}

func (client *Client) LookupTokenMint(ctx context.Context, tokenContract solana.PublicKey) (types.MintAccountInfo, error) {
	var accountInfo types.MintAccountInfo
	info, err := client.SolClient.GetAccountInfoWithOpts(ctx, tokenContract, &rpc.GetAccountInfoOpts{
		Commitment: rpc.CommitmentFinalized,
		Encoding:   "jsonParsed",
	})
	if err != nil {
		return types.MintAccountInfo{}, err
	}
	fmt.Println(string(info.Value.Data.GetRawJSON()))
	err = json.Unmarshal(info.Value.Data.GetRawJSON(), &accountInfo)
	if err != nil {
		return types.MintAccountInfo{}, err
	}
	return accountInfo, nil
}

func (client *Client) LookupTokenAccount(ctx context.Context, tokenAccount solana.PublicKey) (types.TokenAccountInfo, error) {
	var accountInfo types.TokenAccountInfo
	info, err := client.SolClient.GetAccountInfoWithOpts(ctx, tokenAccount, &rpc.GetAccountInfoOpts{
		Commitment: rpc.CommitmentFinalized,
		Encoding:   "jsonParsed",
	})
	if err != nil {
		return types.TokenAccountInfo{}, err
	}
	fmt.Println(string(info.Value.Data.GetRawJSON()))
	accountInfo, err = types.ParseRpcData(info.Value.Data)
	if err != nil {
		return types.TokenAccountInfo{}, err
	}
	return accountInfo, nil
}

func (client *Client) LookupStakeAccount(ctx context.Context, stakeAccount solana.PublicKey) (types.StakeAccount, error) {
	info, err := client.SolClient.GetAccountInfoWithOpts(ctx, stakeAccount, &rpc.GetAccountInfoOpts{
		Commitment: rpc.CommitmentFinalized,
		Encoding:   "jsonParsed",
	})
	if err != nil {
		return types.StakeAccount{}, err
	}
	var stakeAccountInfo types.StakeAccount
	err = json.Unmarshal(info.Value.Data.GetRawJSON(), &stakeAccountInfo)
	if err != nil {
		return types.StakeAccount{}, err
	}
	return stakeAccountInfo, nil
}

type TokenAccountWithInfo struct {
	// We need to manually parse TokenAccountInfo
	Info *types.TokenAccountInfo
	// Account is what's returned by solana client
	Account *rpc.TokenAccount
}

// Get all token accounts for a given token that are owned by an address.
func (client *Client) GetTokenAccountsByOwner(ctx context.Context, addr string, contract string) ([]*TokenAccountWithInfo, error) {
	address, err := solana.PublicKeyFromBase58(addr)
	if err != nil {
		return nil, err
	}
	mint, err := solana.PublicKeyFromBase58(contract)
	if err != nil {
		return nil, err
	}

	conf := rpc.GetTokenAccountsConfig{
		Mint: &mint,
	}
	opts := rpc.GetTokenAccountsOpts{
		Commitment: rpc.CommitmentFinalized,
		// required to be able to parse extra data as json
		Encoding: "jsonParsed",
	}
	out, err := client.SolClient.GetTokenAccountsByOwner(ctx, address, &conf, &opts)
	if err != nil || out == nil {
		return nil, err
	}
	tokenAccounts := []*TokenAccountWithInfo{}
	for _, acc := range out.Value {
		var accountInfo types.TokenAccountInfo
		accountInfo, err = types.ParseRpcData(acc.Account.Data)
		if err != nil {
			return nil, err
		}
		tokenAccounts = append(tokenAccounts, &TokenAccountWithInfo{
			Info:    &accountInfo,
			Account: acc,
		})
	}
	return tokenAccounts, nil
}

// FetchNativeBalance fetches account balance for a Solana address
func (client *Client) FetchNativeBalance(ctx context.Context, address xc.Address) (xc.AmountBlockchain, error) {
	zero := xc.NewAmountBlockchainFromUint64(0)
	out, err := client.SolClient.GetBalance(
		ctx,
		solana.MustPublicKeyFromBase58(string(address)),
		rpc.CommitmentFinalized,
	)
	if err != nil {
		return zero, fmt.Errorf("failed to get balance for '%v': %v", address, err)
	}
	if out == nil {
		return xc.NewAmountBlockchainFromUint64(0), nil
	}

	return xc.NewAmountBlockchainFromUint64(out.Value), nil
}

// FetchBalance fetches token balance for a Solana address
func (client *Client) FetchBalance(ctx context.Context, address xc.Address) (xc.AmountBlockchain, error) {
	return client.FetchBalanceForAsset(ctx, address, client.Asset)
}

// FetchBalanceForAsset fetches a specific token balance which may not be the asset configured for the client
func (client *Client) FetchBalanceForAsset(ctx context.Context, address xc.Address, assetCfg xc.ITask) (xc.AmountBlockchain, error) {
	switch asset := client.Asset.(type) {
	case *xc.ChainConfig:
		return client.FetchNativeBalance(ctx, address)
	case *xc.TokenAssetConfig:
		return client.fetchContractBalance(ctx, address, asset.Contract)
	default:
		contract := asset.GetContract()
		logrus.WithFields(logrus.Fields{
			"chain":      asset.GetChain().Chain,
			"contract":   contract,
			"asset_type": fmt.Sprintf("%T", asset),
		}).Warn("fetching balance for unknown asset type")
		return client.fetchContractBalance(ctx, address, contract)
	}
}

// fetchContractBalance fetches a specific token balance for a Solana address
func (client *Client) fetchContractBalance(ctx context.Context, address xc.Address, contract string) (xc.AmountBlockchain, error) {
	zero := xc.NewAmountBlockchainFromUint64(0)

	allTokenAccounts, err := client.GetTokenAccountsByOwner(ctx, string(address), contract)
	if err != nil {
		return zero, err
	}
	if len(allTokenAccounts) == 0 {
		// if no token accounts then the balance is definitely 0
		return zero, fmt.Errorf("failed to get balance for '%v': %v", address, err)
	}

	totalBal := xc.NewAmountBlockchainFromUint64(0)
	for _, account := range allTokenAccounts {
		bal := xc.NewAmountBlockchainFromStr(account.Info.Parsed.Info.TokenAmount.Amount)
		totalBal = totalBal.Add(&bal)
	}

	return totalBal, nil
}

func (client *Client) FetchDecimals(ctx context.Context, contract xc.ContractAddress) (int, error) {
	if client.Asset.GetChain().IsChain(contract) {
		return int(client.Asset.GetChain().Decimals), nil
	}
	mint, err := solana.PublicKeyFromBase58(string(contract))
	if err != nil {
		return 0, fmt.Errorf("invalid contract address: %s: %v", contract, err)
	}

	mintInfo, err := client.LookupTokenMint(ctx, mint)
	if err != nil {
		return 0, err
	}
	// bz, _ := json.MarshalIndent(mintInfo, "", "  ")
	// fmt.Println(string(bz))

	return int(mintInfo.Parsed.Info.Decimals), nil
}
func (client *Client) FetchBlock(ctx context.Context, args *xclient.BlockArgs) (*xclient.BlockWithTransactions, error) {
	var err error
	height, ok := args.Height()
	if !ok {
		height, err = client.SolClient.GetBlockHeight(ctx, rpc.CommitmentFinalized)
		if err != nil {
			return nil, err
		}
	}
	maxVersion := uint64(0)
	solBlock, err := client.SolClient.GetBlockWithOpts(ctx, height, &rpc.GetBlockOpts{
		MaxSupportedTransactionVersion: &maxVersion,
	})
	if err != nil {
		return nil, err
	}
	if solBlock.BlockHeight != nil {
		height = *solBlock.BlockHeight
	}
	blockTime := time.Unix(0, 0)
	if solBlock.BlockTime != nil {
		blockTime = solBlock.BlockTime.Time()
	}
	block := &xclient.BlockWithTransactions{
		Block: *xclient.NewBlock(client.Asset.GetChain().Chain, height, solBlock.Blockhash.String(), blockTime),
	}
	for _, tx := range solBlock.Transactions {
		parsed, err := tx.GetTransaction()
		// Should we just skip it?
		if err != nil {
			return nil, fmt.Errorf("could not parsed tx in block: %v", err)
		}
		block.TransactionIds = append(block.TransactionIds, parsed.Signatures[0].String())
	}
	return block, nil

}
