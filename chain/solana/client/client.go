package client

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/cordialsys/crosschain/client/errors"
	txinfo "github.com/cordialsys/crosschain/client/tx_info"

	xc "github.com/cordialsys/crosschain"
	bin "github.com/gagliardetto/binary"
	lookup "github.com/gagliardetto/solana-go/programs/address-lookup-table"
	"github.com/sirupsen/logrus"

	xcbuilder "github.com/cordialsys/crosschain/builder"
	"github.com/cordialsys/crosschain/chain/solana/builder"
	"github.com/cordialsys/crosschain/chain/solana/client/solscan"
	"github.com/cordialsys/crosschain/chain/solana/tx"
	"github.com/cordialsys/crosschain/chain/solana/tx_input"
	"github.com/cordialsys/crosschain/chain/solana/types"
	xclient "github.com/cordialsys/crosschain/client"
	xctypes "github.com/cordialsys/crosschain/client/types"
	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/rpc"
)

// Client for Solana
type Client struct {
	SolClient *rpc.Client
	Asset     *xc.ChainConfig
}

var _ xclient.Client = &Client{}
var _ xclient.StakingClient = &Client{}
var _ xclient.CallClient = &Client{}

// NewClient returns a new JSON-RPC Client to the Solana node
func NewClient(cfgI *xc.ChainConfig) (*Client, error) {
	cfg := cfgI.GetChain()
	solClient := rpc.New(cfg.URL)
	return &Client{
		SolClient: solClient,
		Asset:     cfgI,
	}, nil
}

// DeriveNonceAccount derives a deterministic nonce account address from a sender's public key.
func DeriveNonceAccount(from solana.PublicKey) (solana.PublicKey, error) {
	return solana.CreateWithSeed(from, builder.DurableNonceSeed, solana.SystemProgramID)
}

func (client *Client) FetchBaseInput(ctx context.Context, fromAddr xc.Address) (*tx_input.TxInput, error) {
	txInput := tx_input.NewTxInput()

	// get recent block hash (always needed as fallback and for nonce account creation)
	recent, err := client.SolClient.GetLatestBlockhash(ctx, rpc.CommitmentFinalized)
	if err != nil {
		return nil, fmt.Errorf("could not get latest blockhash: %v", err)
	}
	if recent == nil || recent.Value == nil {
		return nil, fmt.Errorf("error fetching latest blockhash")
	}
	txInput.RecentBlockHash = recent.Value.Blockhash
	// fixed 5000 lamports
	// https://solana.com/docs/core/fees#key-points
	txInput.BaseFee = xc.NewAmountBlockchainFromUint64(5000)

	// Derive and check for a durable nonce account
	fromPub, err := solana.PublicKeyFromBase58(string(fromAddr))
	if err != nil {
		return nil, fmt.Errorf("invalid from address: %v", err)
	}
	nonceAccountPub, err := DeriveNonceAccount(fromPub)
	if err != nil {
		return nil, fmt.Errorf("could not derive nonce account: %v", err)
	}
	err = client.FetchDurableNonceInput(ctx, txInput, nonceAccountPub)
	if err != nil {
		return nil, fmt.Errorf("could not fetch durable nonce: %v", err)
	}

	return txInput, nil
}

// NonceAccountState represents the on-chain state of a durable nonce account.
type NonceAccountState struct {
	AuthorizedPubkey solana.PublicKey
	Nonce            solana.Hash
	FeeCalculator    struct {
		LamportsPerSignature uint64
	}
}

// The nonce account data layout (after the 4-byte version/state prefix):
// 4 bytes: version (0 for legacy, 1 for current)
// 4 bytes: state (0 = uninitialized, 1 = initialized)
// 32 bytes: authority pubkey
// 32 bytes: nonce (blockhash)
// 8 bytes: lamports per signature
const nonceAccountDataSize = 80

// FetchNonceAccount retrieves the current state of a durable nonce account.
func (client *Client) FetchNonceAccount(ctx context.Context, nonceAccountAddr solana.PublicKey) (*NonceAccountState, error) {
	info, err := client.SolClient.GetAccountInfo(ctx, nonceAccountAddr)
	if err != nil {
		return nil, fmt.Errorf("could not get nonce account info: %v", err)
	}
	if info == nil || info.Value == nil {
		return nil, fmt.Errorf("nonce account not found: %s", nonceAccountAddr)
	}

	data := info.Value.Data.GetBinary()
	if len(data) < nonceAccountDataSize {
		return nil, fmt.Errorf("nonce account data too small: %d bytes", len(data))
	}

	// Parse nonce account data
	// Bytes 0-3: version (uint32 LE)
	// Bytes 4-7: state (uint32 LE) -- 1 = initialized
	// Bytes 8-39: authority pubkey (32 bytes)
	// Bytes 40-71: nonce/blockhash (32 bytes)
	// Bytes 72-79: lamports per signature (uint64 LE)
	state := &NonceAccountState{}

	stateVal := uint32(data[4]) | uint32(data[5])<<8 | uint32(data[6])<<16 | uint32(data[7])<<24
	if stateVal != 1 {
		return nil, fmt.Errorf("nonce account is not initialized (state=%d)", stateVal)
	}

	copy(state.AuthorizedPubkey[:], data[8:40])
	copy(state.Nonce[:], data[40:72])
	state.FeeCalculator.LamportsPerSignature = uint64(data[72]) | uint64(data[73])<<8 |
		uint64(data[74])<<16 | uint64(data[75])<<24 | uint64(data[76])<<32 |
		uint64(data[77])<<40 | uint64(data[78])<<48 | uint64(data[79])<<56

	return state, nil
}

// FetchDurableNonceInput populates the TxInput with durable nonce information.
// If the nonce account exists and is initialized, it reads the current nonce value.
// If the nonce account doesn't exist, it sets ShouldCreateDurableNonce=true.
func (client *Client) FetchDurableNonceInput(ctx context.Context, txInput *tx_input.TxInput, nonceAccount solana.PublicKey) error {
	txInput.DurableNonceAccount = nonceAccount

	nonceState, err := client.FetchNonceAccount(ctx, nonceAccount)
	if err != nil {
		// If account doesn't exist or is not initialized, mark it for creation
		txInput.ShouldCreateDurableNonce = true
		logrus.WithField("nonce_account", nonceAccount.String()).WithError(err).Info("nonce account not found or not initialized, will need setup")
		return nil
	}

	txInput.DurableNonce = nonceState.Nonce
	return nil
}

// FetchLegacyTxInput returns tx input for a Solana tx, namely a RecentBlockHash
func (client *Client) FetchTransferInput(ctx context.Context, args xcbuilder.TransferArgs) (xc.TxInput, error) {
	txInput, err := client.FetchBaseInput(ctx, args.GetFrom())
	if err != nil {
		return nil, err
	}

	contract, _ := args.GetContract()
	if contract == "" {
		// native transfer
		return client.WithTransferSimulation(ctx, args, txInput)
	}

	mint, err := solana.PublicKeyFromBase58(string(contract))
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
		ataToStr, err := types.FindAssociatedTokenAddress(string(args.GetTo()), string(contract), mintInfo.Value.Owner)
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
		tokenAccounts, err := client.GetTokenAccountsByOwner(ctx, string(args.GetFrom()), string(contract))
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
	txInput.PrioritizationFee = txInput.PrioritizationFee.ApplyGasPriceMultiplier(client.Asset.GetChain().Client())

	return client.WithTransferSimulation(ctx, args, txInput)
}

func (client *Client) WithTransferSimulation(ctx context.Context, args xcbuilder.TransferArgs, txInput *tx_input.TxInput) (xc.TxInput, error) {
	builder, err := builder.NewTxBuilder(client.Asset.GetChain().Base())
	if err != nil {
		return &tx_input.TxInput{}, fmt.Errorf("could not create tx builder: %v", err)
	}
	txI, err := builder.Transfer(args, txInput)
	if err != nil {
		return &tx_input.TxInput{}, fmt.Errorf("could not create tx builder: %v", err)
	}
	tx := txI.(*tx.Tx)
	tx.SolTx.Signatures = []solana.Signature{
		// one signature for solana transfers (note: staking txs use multiple)
		{},
	}
	if _, ok := args.GetFeePayer(); ok {
		// add another for the fee payer
		tx.SolTx.Signatures = append(tx.SolTx.Signatures, solana.Signature{})
	}

	sim, err := client.SolClient.SimulateTransactionWithOpts(ctx, tx.SolTx, &rpc.SimulateTransactionOpts{
		SigVerify: false,
	})

	// sim, err := client.SolClient.SimulateTransaction(ctx, tx.SolTx)
	if err != nil {
		return &tx_input.TxInput{}, fmt.Errorf("could not simulate tx: %v", err)
	}
	// simBz, _ := json.MarshalIndent(sim, "", "  ")
	// fmt.Println(string(simBz))
	if sim.Value != nil && sim.Value.UnitsConsumed != nil {
		txInput.UnitsConsumed = *sim.Value.UnitsConsumed
	}
	return txInput, nil
}

func (client *Client) SubmitTx(ctx context.Context, txInput xctypes.SubmitTxReq) error {
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

// Taken from solana-go README.md example
// https://github.com/gagliardetto/solana-go?tab=readme-ov-file#address-lookup-tables
func processTransactionWithAddressLookups(ctx context.Context, txx *solana.Transaction, rpcClient *rpc.Client) error {
	if !txx.Message.IsVersioned() {
		return nil
	}
	tblKeys := txx.Message.GetAddressTableLookups().GetTableIDs()
	if len(tblKeys) == 0 {
		return nil
	}
	numLookups := txx.Message.GetAddressTableLookups().NumLookups()
	if numLookups == 0 {
		return nil
	}
	resolutions := make(map[solana.PublicKey]solana.PublicKeySlice)
	for _, key := range tblKeys {
		info, err := rpcClient.GetAccountInfo(
			ctx,
			key,
		)
		if err != nil {
			return err
		}
		tableContent, err := lookup.DecodeAddressLookupTableState(info.GetBinary())
		if err != nil {
			return err
		}

		resolutions[key] = tableContent.Addresses
	}

	err := txx.Message.SetAddressTables(resolutions)
	if err != nil {
		return err
	}

	err = txx.Message.ResolveLookups()
	if err != nil {
		return err
	}
	// fmt.Println(txx.String())
	return nil
}

// FetchLegacyTxInfo returns tx info for a Solana tx
// RPC is always attempted first, then indexer fallback if configured.
func (client *Client) FetchLegacyTxInfo(ctx context.Context, txHash xc.TxHash) (txinfo.LegacyTxInfo, error) {
	// Keep input validation behavior the same regardless of data source.
	if _, err := solana.SignatureFromBase58(string(txHash)); err != nil {
		return txinfo.LegacyTxInfo{}, err
	}

	rpcInfo, rpcErr := client.fetchLegacyTxInfoFromRPC(ctx, txHash)
	if rpcErr == nil {
		return rpcInfo, nil
	}

	indexerURL := strings.TrimSpace(client.Asset.GetChain().IndexerUrl)
	if indexerURL == "" {
		return txinfo.LegacyTxInfo{}, rpcErr
	}

	solscanClient, err := solscan.NewClient(indexerURL)
	if err != nil {
		return txinfo.LegacyTxInfo{}, err
	}

	indexerInfo, indexerErr := solscanClient.GetLegacyTxInfo(ctx, string(txHash))
	if indexerErr == nil {
		return indexerInfo, indexerErr
	}

	logrus.WithFields(logrus.Fields{
		"tx_hash":       txHash,
		"rpc_error":     rpcErr.Error(),
		"indexer_url":   indexerURL,
		"indexer_error": indexerErr.Error(),
	}).Warn("solana tx-info lookup failed on rpc and indexer fallback")

	// Preserve RPC behavior when both fail by returning the RPC error.
	return txinfo.LegacyTxInfo{}, rpcErr
}

func (client *Client) fetchLegacyTxInfoFromRPC(ctx context.Context, txHash xc.TxHash) (txinfo.LegacyTxInfo, error) {
	result := txinfo.LegacyTxInfo{}

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
		return result, fmt.Errorf("error decoding transaction: %w", err)
	}

	// Complicated txs may contain only pointers to address, forcing us to resolve the accounts/addresses stored on-chain.
	err = processTransactionWithAddressLookups(ctx, solTx, client.SolClient)
	if err != nil {
		return result, errors.TransactionNotFoundf("error resolving address-lookups: %v", err)
	}

	decoder := tx.NewDecoderFromNativeTx(solTx, res.Meta)
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
	totalFee := meta.Fee
	// Include nonce account creation rent as part of the fee.
	// When a durable nonce account is created (InitializeNonceAccount), the rent-exempt
	// lamports paid via CreateAccountWithSeed should be attributed as a fee.
	for _, nonceInit := range decoder.GetInitializeNonceAccounts() {
		nonceAccountKey := nonceInit.Instruction.GetNonceAccount().PublicKey
		for _, createAccount := range decoder.GetCreateAccounts() {
			if createAccount.Instruction.NewAccount.Equals(nonceAccountKey) {
				totalFee += createAccount.Instruction.Lamports
			}
		}
	}
	result.Fee = xc.NewAmountBlockchainFromUint64(totalFee)
	accountKeys := decoder.GetAccountKeys()
	if len(accountKeys) > 0 {
		// The first account is the fee payer on solana
		result.FeePayer = xc.Address(accountKeys[0].String())
	}

	result.TxID = string(txHash)

	accountResolver := NewSolanaTokenAccountResolver(client.SolClient)
	sources, dests, err := BuildTransfersFromDecoder(
		ctx,
		client.SolClient,
		decoder,
		accountResolver,
		client.Asset.GetChain().Chain,
	)
	if err != nil {
		return result, err
	}

	for _, instr := range decoder.GetDelegateStake() {
		xcStake := &txinfo.Stake{
			Account:   instr.Instruction.GetStakeAccount().PublicKey.String(),
			Validator: instr.Instruction.GetVoteAccount().PublicKey.String(),
			Address:   instr.Instruction.GetStakeAuthority().PublicKey.String(),
			// Needs to be looked up from separate instruction
			Balance: xc.AmountBlockchain{},
		}
		for _, createAccount := range decoder.GetCreateAccounts() {
			if createAccount.Instruction.NewAccount.Equals(instr.Instruction.GetStakeAccount().PublicKey) {
				xcStake.Balance = xc.NewAmountBlockchainFromUint64(createAccount.Instruction.Lamports)
			}
		}

		result.AddStakeEvent(xcStake)
	}
	for _, instr := range decoder.GetDeactivateStakes() {
		xcStake := &txinfo.Unstake{
			Account: instr.Instruction.GetStakeAccount().PublicKey.String(),
			Address: instr.Instruction.GetStakeAuthority().PublicKey.String(),

			// Needs to be looked up
			Balance:   xc.AmountBlockchain{},
			Validator: "",
		}
		stakeAccountInfo, err := client.LookupStakeAccount(ctx, instr.Instruction.GetStakeAccount().PublicKey)
		if err != nil {
			logrus.WithError(err).Warn("failed to lookup stake account")
		} else {
			xcStake.Validator = stakeAccountInfo.Parsed.Info.Stake.Delegation.Voter
			xcStake.Balance = xc.NewAmountBlockchainFromStr(stakeAccountInfo.Parsed.Info.Stake.Delegation.Stake)
		}
		result.AddStakeEvent(xcStake)
	}

	for i, instr := range decoder.GetMemos() {
		message := instr.Instruction.Message
		if i < len(dests) {
			// Report nth memo to nth movement.
			// Shouldn't matter if we assign to dest or source here, as they are the same movement.
			dests[i].Memo = string(message)
		}
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

func (client *Client) FetchTxInfo(ctx context.Context, args *txinfo.Args) (txinfo.TxInfo, error) {
	txHashStr := args.TxHash()
	legacyTx, err := client.FetchLegacyTxInfo(ctx, txHashStr)
	if err != nil {
		return txinfo.TxInfo{}, err
	}

	// remap to new tx
	return txinfo.TxInfoFromLegacy(client.Asset.GetChain(), legacyTx, txinfo.Account), nil
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
	// fmt.Println(string(info.Value.Data.GetRawJSON()))
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
	// fmt.Println(string(info.Value.Data.GetRawJSON()))
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
func (client *Client) FetchBalance(ctx context.Context, args *xclient.BalanceArgs) (xc.AmountBlockchain, error) {
	return client.FetchBalanceForAsset(ctx, args)
}

func (client *Client) FetchBalanceForAsset(ctx context.Context, args *xclient.BalanceArgs) (xc.AmountBlockchain, error) {

	if contract, ok := args.Contract(); ok {
		return client.fetchContractBalance(ctx, args.Address(), string(contract))
	} else {
		return client.FetchNativeBalance(ctx, args.Address())
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
func (client *Client) FetchBlock(ctx context.Context, args *xclient.BlockArgs) (*txinfo.BlockWithTransactions, error) {
	var err error
	height, ok := args.Height()
	if !ok {
		height, err = client.SolClient.GetSlot(ctx, rpc.CommitmentFinalized)
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
	height = solBlock.ParentSlot + 1
	blockTime := time.Unix(0, 0)
	if solBlock.BlockTime != nil {
		blockTime = solBlock.BlockTime.Time()
	}
	block := &txinfo.BlockWithTransactions{
		Block: *txinfo.NewBlock(client.Asset.GetChain().Chain, height, solBlock.Blockhash.String(), blockTime),
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
