package solana

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"sort"
	"time"

	xc "github.com/cordialsys/crosschain"
	"github.com/shopspring/decimal"
	"github.com/sirupsen/logrus"

	xcbuilder "github.com/cordialsys/crosschain/builder"
	xclient "github.com/cordialsys/crosschain/client"
	bin "github.com/gagliardetto/binary"
	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/rpc"
)

// type TokenType string

// var TokenSpl TokenType = "spl"
// var Token2022 TokenType = "token2022"

// TxInput for Solana
type TxInput struct {
	xc.TxInputEnvelope
	RecentBlockHash     solana.Hash         `json:"recent_block_hash,omitempty"`
	ToIsATA             bool                `json:"to_is_ata,omitempty"`
	TokenProgram        solana.PublicKey    `json:"token_program"`
	ShouldCreateATA     bool                `json:"should_create_ata,omitempty"`
	SourceTokenAccounts []*TokenAccount     `json:"source_token_accounts,omitempty"`
	PrioritizationFee   xc.AmountBlockchain `json:"prioritization_fee,omitempty"`
	Timestamp           int64               `json:"timestamp,omitempty"`
}

var _ xc.TxInput = &TxInput{}
var _ xc.TxInputWithUnix = &TxInput{}

// Solana recent-block-hash timeout margin
const SafetyTimeoutMargin = (5 * time.Minute)

// Returns the microlamports to set the compute budget unit price.
// It will not go about the max price amount for safety concerns.
func (input *TxInput) GetLimitedPrioritizationFee(chain *xc.ChainConfig) uint64 {
	fee := input.PrioritizationFee.Uint64()
	max := uint64(chain.ChainMaxGasPrice)
	if max == 0 {
		// set default max price to spend max 1 SOL on a transaction
		// 1 SOL = (1 * 10 ** 9) * 10 ** 6 microlamports
		// /200_000 compute units
		max = 5_000_000_000
	}
	if fee > max {
		fee = max
	}
	return fee
}

func (input *TxInput) SetGasFeePriority(other xc.GasFeePriority) error {
	multiplier, err := other.GetDefault()
	if err != nil {
		return err
	}
	multipliedFee := multiplier.Mul(decimal.NewFromBigInt(input.PrioritizationFee.Int(), 0)).BigInt()
	input.PrioritizationFee = xc.AmountBlockchain(*multipliedFee)
	return nil
}

func (input *TxInput) IndependentOf(other xc.TxInput) (independent bool) {
	// no conflicts on solana as txs are easily parallelizeable through
	// the recent-block-hash mechanism.
	return true
}

func (input *TxInput) SafeFromDoubleSend(others ...xc.TxInput) (safe bool) {
	if !xc.SameTxInputTypes(input, others...) {
		return false
	}
	for _, other := range others {
		oldInput, ok := other.(*TxInput)
		if ok {
			diff := input.Timestamp - oldInput.Timestamp
			// solana blockhash lasts only ~1 minute -> we'll require a 5 min period
			// and different hash to consider it safe from double-send.
			if diff < int64(SafetyTimeoutMargin.Seconds()) || oldInput.RecentBlockHash.Equals(input.RecentBlockHash) {
				// not yet safe
				return false
			}
		} else {
			// can't tell (this shouldn't happen) - default false
			return false
		}
	}
	// all timed out - we're safe
	return true
}

func (input *TxInput) SetUnix(unix int64) {
	input.Timestamp = unix
}

type TokenAccount struct {
	Account solana.PublicKey    `json:"account,omitempty"`
	Balance xc.AmountBlockchain `json:"balance,omitempty"`
}

// NewTxInput returns a new Solana TxInput
func NewTxInput() *TxInput {
	return &TxInput{
		TxInputEnvelope: *xc.NewTxInputEnvelope(xc.DriverSolana),
	}
}

// Client for Solana
type Client struct {
	SolClient *rpc.Client
	Asset     xc.ITask
}

var _ xclient.FullClient = &Client{}

// NewClient returns a new JSON-RPC Client to the Solana node
func NewClient(cfgI xc.ITask) (*Client, error) {
	cfg := cfgI.GetChain()
	solClient := rpc.New(cfg.URL)
	return &Client{
		SolClient: solClient,
		Asset:     cfgI,
	}, nil
}

// FetchLegacyTxInput returns tx input for a Solana tx, namely a RecentBlockHash
func (client *Client) FetchTransferInput(ctx context.Context, args xcbuilder.TransferArgs) (xc.TxInput, error) {
	txInput := NewTxInput()
	asset := client.Asset

	// get recent block hash (i.e. nonce)
	// GetRecentBlockhash will be deprecated - GetLatestBlockhash already tested, just switch
	// recent, err := client.SolClient.GetLatestBlockhash(ctx, rpc.CommitmentFinalized)
	recent, err := client.SolClient.GetRecentBlockhash(ctx, rpc.CommitmentFinalized)
	if err != nil {
		return nil, err
	}
	if recent == nil || recent.Value == nil {
		return nil, errors.New("error fetching blockhash")
	}
	txInput.RecentBlockHash = recent.Value.Blockhash
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
		ataToStr, err := FindAssociatedTokenAddress(string(args.GetTo()), contract, mintInfo.Value.Owner)
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
				txInput.SourceTokenAccounts = append(txInput.SourceTokenAccounts, &TokenAccount{
					Account: acc.Account.Pubkey,
					Balance: amount,
				})
			}
		}

		// To prevent dust issues, we sort descending and limit number of token accounts
		sort.Slice(txInput.SourceTokenAccounts, func(i, j int) bool {
			return txInput.SourceTokenAccounts[i].Balance.Cmp(&txInput.SourceTokenAccounts[j].Balance) > 0
		})
		if len(txInput.SourceTokenAccounts) > MaxTokenTransfers {
			txInput.SourceTokenAccounts = txInput.SourceTokenAccounts[:MaxTokenTransfers]
		}

		if len(tokenAccounts) == 0 {
			// no balance
			return nil, errors.New("no balance to send solana token")
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
		return result, err
	}
	if res == nil || res.Transaction == nil {
		return result, errors.New("invalid transaction in response")
	}

	solTx, err := solana.TransactionFromDecoder(bin.NewBinDecoder(res.Transaction.GetBinary()))
	if err != nil {
		return result, err
	}
	tx := NewTxFrom(solTx)
	meta := res.Meta
	if res.BlockTime != nil {
		result.BlockTime = res.BlockTime.Time().Unix()
	}

	if res.Slot > 0 {
		result.BlockIndex = int64(res.Slot)
		if res.BlockTime != nil {
			result.BlockTime = int64(*res.BlockTime)
		}

		recent, err := client.SolClient.GetRecentBlockhash(ctx, rpc.CommitmentFinalized)
		if err != nil {
			// ignore
		} else {
			result.Confirmations = int64(recent.Context.Slot) - result.BlockIndex
		}
	}
	result.Fee = xc.NewAmountBlockchainFromUint64(meta.Fee)

	result.TxID = string(txHash)
	result.ExplorerURL = client.Asset.GetChain().ExplorerURL + "/tx/" + result.TxID + "?cluster=" + client.Asset.GetChain().Net
	result.ContractAddress = tx.ContractAddress()

	toAddr := tx.ToOwnerAccount()
	// If no clear destination, try looking up owner behind a token account
	if toAddr == "" {
		// check ATA
		tokenAccount, ok := tx.ToTokenAccount()
		if ok {
			tokenAccountInfo, err := client.LookupTokenAccount(ctx, tokenAccount)
			if err != nil {
				// pass
			} else {
				toAddr = xc.Address(tokenAccountInfo.Parsed.Info.Owner)
				result.ContractAddress = xc.ContractAddress(tokenAccountInfo.Parsed.Info.Mint)
			}
		}
	}

	result.From = tx.From()
	result.To = toAddr
	result.Amount = tx.Amount()

	return result, nil
}

func (client *Client) FetchTxInfo(ctx context.Context, txHashStr xc.TxHash) (xclient.TxInfo, error) {
	legacyTx, err := client.FetchLegacyTxInfo(ctx, txHashStr)
	if err != nil {
		return xclient.TxInfo{}, err
	}

	// remap to new tx
	return xclient.TxInfoFromLegacy(client.Asset.GetChain().Chain, legacyTx, xclient.Account), nil
}

func (client *Client) LookupTokenAccount(ctx context.Context, tokenAccount solana.PublicKey) (TokenAccountInfo, error) {
	var accountInfo TokenAccountInfo
	info, err := client.SolClient.GetAccountInfoWithOpts(ctx, tokenAccount, &rpc.GetAccountInfoOpts{
		Commitment: rpc.CommitmentFinalized,
		Encoding:   "jsonParsed",
	})
	if err != nil {
		return TokenAccountInfo{}, err
	}
	accountInfo, err = ParseRpcData(info.Value.Data)
	if err != nil {
		return TokenAccountInfo{}, err
	}
	return accountInfo, nil
}

// FindAssociatedTokenAddress returns the associated token account (ATA) for a given account and token
func FindAssociatedTokenAddress(addr string, contract string, tokenProgram solana.PublicKey) (string, error) {
	address, err := solana.PublicKeyFromBase58(addr)
	if err != nil {
		return "", err
	}
	mint, err := solana.PublicKeyFromBase58(contract)
	if err != nil {
		return "", err
	}
	if len(tokenProgram) == 0 || tokenProgram.IsZero() {
		tokenProgram = solana.TokenProgramID
	}
	associatedAddr, _, err := solana.FindProgramAddress(
		[][]byte{
			address[:],
			tokenProgram[:],
			mint[:],
		},
		solana.SPLAssociatedTokenAccountProgramID,
	)
	if err != nil {
		return "", err
	}
	return associatedAddr.String(), nil
}

type TokenAccountWithInfo struct {
	// We need to manually parse TokenAccountInfo
	Info *TokenAccountInfo
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
		var accountInfo TokenAccountInfo
		accountInfo, err = ParseRpcData(acc.Account.Data)
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
