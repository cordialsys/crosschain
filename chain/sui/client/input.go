package client

import (
	"context"
	"crypto/rand"
	"encoding/binary"
	"errors"
	"fmt"

	xc "github.com/cordialsys/crosschain"
	xcbuilder "github.com/cordialsys/crosschain/builder"
	"github.com/cordialsys/crosschain/chain/sui"
	pb "github.com/cordialsys/crosschain/chain/sui/client/rpc"
	"github.com/cordialsys/go-sui-sdk/v2/types"
	"github.com/google/uuid"
	"google.golang.org/protobuf/proto"
)

// Inputs deliberately retain sui.TxInput and its legacy coin JSON encoding.
func (c *Client) fetchBaseInput(ctx context.Context, contract string, from xc.Address, feePayer xc.Address) (*sui.TxInput, error) {
	native := c.Asset.GetChain().ChainCoin
	if native == "" {
		native = sui.NativeCoin
	}
	if contract == "" {
		contract = native
	}
	contract = sui.NormalizeCoinContract(contract)

	input := sui.NewTxInput()
	if feePayer == from {
		feePayer = ""
	}
	gasOwner := from
	if feePayer != "" {
		gasOwner = feePayer
	}
	input.GasCoinOwner = gasOwner
	addressBalance, err := c.addressBalance(ctx, from, contract)
	if err != nil {
		return nil, fmt.Errorf("failed to get address balance: %w", err)
	}
	gasBalance := addressBalance
	if gasOwner != from || !sameCoin(contract, native) {
		gasBalance, err = c.addressBalance(ctx, gasOwner, native)
		if err != nil {
			return nil, fmt.Errorf("failed to get gas address balance: %w", err)
		}
	}

	suiCoins, err := c.GetAllCoinsFor(ctx, from, native)
	if err != nil {
		return nil, fmt.Errorf("failed to get coins: %w", err)
	}
	sui.SortCoins(suiCoins)
	if len(suiCoins) > 0 {
		// use the largest SUI coin for gas
		input.GasCoin = *suiCoins[0]
		input.GasCoinOwner = from
	}

	gasCoins := suiCoins
	if feePayer != "" {
		input.GasCoin = types.Coin{}
		sponsorSuiCoins, err := c.GetAllCoinsFor(ctx, feePayer, native)
		if err != nil {
			return nil, fmt.Errorf("failed to get fee payer coins: %w", err)
		}
		gasCoins = sponsorSuiCoins
		// use gas coin from sponsor if available
		sui.SortCoins(sponsorSuiCoins)
		if len(sponsorSuiCoins) > 0 {
			input.GasCoin = *sponsorSuiCoins[0]
			input.GasCoinOwner = feePayer
		}
	}

	input.GasCoinOwner = gasOwner
	if len(input.GasCoin.Digest) == 0 && gasBalance == 0 {
		return nil, fmt.Errorf("no SUI coin objects or address balance available for gas")
	}
	// Multiple small gas coins must be smashed before execution, even when
	// there is no address balance. The legacy builder only uses one gas coin.
	if addressBalance > 0 || gasBalance > 0 || len(gasCoins) > 1 {
		info, err := c.Ledger.GetServiceInfo(ctx, &pb.GetServiceInfoRequest{})
		if err != nil {
			return nil, fmt.Errorf("failed to get chain ID: %w", err)
		}
		if info.GetChainId() == "" {
			return nil, fmt.Errorf("Sui returned no chain ID")
		}
		var nonce [4]byte
		if _, err = rand.Read(nonce[:]); err != nil {
			return nil, err
		}
		if sameCoin(contract, native) {
			contract = sui.NativeCoin
		}
		if len(gasCoins) > sui.MaxCoinObjects {
			gasCoins = gasCoins[:sui.MaxCoinObjects]
		}
		input.AddressBalance = &sui.AddressBalanceInput{Sender: from, CoinType: contract, Balance: addressBalance, GasBalance: gasBalance, GasCoins: append([]*types.Coin(nil), gasCoins...), ChainID: info.GetChainId(), Nonce: binary.LittleEndian.Uint32(nonce[:])}
	}

	transferCoins := suiCoins
	if !sameCoin(contract, native) {
		// If we're not sending SUI, we need to make a separate call to get coins that are being transferred.
		transferCoins, err = c.GetAllCoinsFor(ctx, from, contract)
		if err != nil {
			return nil, fmt.Errorf("failed to get contract coins: %w", err)
		}
		sui.SortCoins(transferCoins)
	}

	latestCheckpoint, err := c.FetchLatestCheckpoint(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch latest checkpoint: %w", err)
	}
	epoch := xc.NewAmountBlockchainFromStr(latestCheckpoint.Epoch)
	input.CurrentEpoch = epoch.Uint64()

	// store the object id's for the transfer
	input.Coins = transferCoins
	input.SortCoins()
	// take max 50 to bound the tx_input size.
	if len(input.Coins) > sui.MaxCoinObjects {
		input.Coins = input.Coins[:sui.MaxCoinObjects]
	}

	gasPrice, err := c.EstimateGas(ctx)
	if err != nil {
		defaultgas := c.Asset.GetChain().ChainGasPriceDefault
		if defaultgas < 0.1 {
			return input, fmt.Errorf("failed to estimate gas: %w", err)
		}
		// use the default
		input.GasPrice = uint64(defaultgas)
	} else {
		input.GasPrice = gasPrice.Uint64()
	}
	// 2 SUI
	input.GasBudget = 2_000_000_000

	// Incrementally increase budget per additional coin being consumed
	input.GasBudget = input.GasBudget + sui.GAS_BUDGET_PER_COIN*uint64(len(input.Coins))

	input.ExcludeGasCoin()
	return input, nil
}

func (c *Client) FetchTransferInput(ctx context.Context, args xcbuilder.TransferArgs) (xc.TxInput, error) {
	amount := args.GetAmount()
	_, hasID := args.GetTransactionID()
	if id, ok := args.GetTransactionID(); ok && id == "" {
		return nil, fmt.Errorf("transaction ID must not be empty")
	}
	if amount.Sign() < 0 || amount.Int().BitLen() > 64 {
		return nil, fmt.Errorf("Sui transfer amount must fit uint64")
	}

	native := xc.ContractAddress(c.Asset.GetChain().ChainCoin)
	if native == "" {
		native = sui.NativeCoin
	}
	contract, _ := args.GetContract()
	isNative := contract == "" || sameCoin(string(contract), string(native))
	feePayer, _ := args.GetFeePayer()
	input, err := c.fetchBaseInput(ctx, string(contract), args.GetFrom(), feePayer)
	if err != nil {
		return input, fmt.Errorf("failed to fetch base input: %w", err)
	}
	coinType := string(contract)
	if isNative {
		coinType = sui.NativeCoin
	}
	input.PaymentKit, err = c.fetchPaymentKit(ctx, args.GetFrom(), coinType, hasID)
	if err != nil {
		return nil, err
	}
	// Without an explicit ID keep the coins available for the unprotected build.
	if hasID {
		input.PreferAddressBalance(amount)
	}
	if _, ok := args.GetPublicKey(); !ok {
		args.SetPublicKey(make([]byte, 32))
	}

	builder, err := sui.NewTxBuilder(c.Asset.GetChain().Base())
	if err != nil {
		return input, fmt.Errorf("could not create tx builder: %w", err)
	}

	// Estimate the registry write with a fresh throwaway ID. Using the real
	// ID here would make a previously executed payment fail before applications
	// can apply their ID and check history. This ID never leaves the simulation.
	simulationArgs := args
	if input.PaymentKit != nil {
		simulationArgs.SetTransactionID(uuid.NewString())
	}
	initialBudget := input.GasBudget
	err = c.estimateAndValidate(ctx, input, isNative, func() (xc.Tx, error) { return builder.Transfer(simulationArgs, input) })
	if err != nil {
		return nil, err
	}

	// Try an address-only plan on a copy: its larger storage charge may
	// exceed a small address balance even though coin funding is affordable.
	candidate := input.Clone()
	if hasID && candidate.PreferAddressBalance(amount) {
		candidate.GasBudget = initialBudget
		candidateErr := c.estimateAndValidate(ctx, candidate, isNative, func() (xc.Tx, error) { return builder.Transfer(simulationArgs, candidate) })
		if candidateErr != nil && !errors.Is(candidateErr, errInsufficientGas) {
			return nil, fmt.Errorf("estimate address-funded payment: %w", candidateErr)
		}
		// Keep the already validated coin plan if address funds cannot cover gas.
		if candidateErr == nil {
			input = candidate
		}
	}

	if !hasID && input.PaymentKit != nil {
		// The application may either set an ID after fetching or build directly
		// without one. Retain the coins and budget for both funding paths: using
		// coin gas can cost more than the simulated address-funded payment.
		coinInput := input.Clone()
		coinInput.GasBudget = initialBudget
		coinTx, buildErr := builder.Transfer(args, coinInput)
		if buildErr == nil {
			coinFee, err := c.simulateTransactionGasFee(ctx, coinTx, isNative)
			if err != nil {
				return nil, fmt.Errorf("estimate coin-funded transfer without ID: %w", err)
			}
			if coinInput.GasBudget < coinFee {
				return nil, fmt.Errorf("insufficient coin gas without a transaction ID: need %d, have %d", coinFee, coinInput.GasBudget)
			}
			input.GasBudgetUpperBound = input.GasBudget != coinFee
			input.GasBudget = max(input.GasBudget, coinFee)
		}
		// An address-only input is still useful to applications that apply the
		// trusted ID setter. A build without that ID will reject its lack of coins.
	}

	return input, nil
}

// An absent address_balance is zero on older coin-only networks. A missing
// balance object is a malformed response, not an empty account.
func (c *Client) addressBalance(ctx context.Context, owner xc.Address, coin string) (uint64, error) {
	response, err := c.State.GetBalance(ctx, &pb.GetBalanceRequest{Owner: proto.String(string(owner)), CoinType: proto.String(coin)})
	if err != nil {
		return 0, err
	}
	if response.GetBalance() == nil {
		return 0, fmt.Errorf("Sui returned no balance")
	}
	return response.GetBalance().GetAddressBalance(), nil
}
