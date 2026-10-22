package client

import (
	"context"
	"crypto/rand"
	"encoding/binary"
	"fmt"
	pb "github.com/cordialsys/crosschain/chain/sui/client/rpc"
	"google.golang.org/protobuf/proto"

	xc "github.com/cordialsys/crosschain"
	xcbuilder "github.com/cordialsys/crosschain/builder"
	"github.com/cordialsys/crosschain/chain/sui"
	"github.com/cordialsys/go-sui-sdk/v2/types"
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
	if addressBalance > 0 || gasBalance > 0 {
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
	if _, ok := args.GetPublicKey(); !ok {
		args.SetPublicKey(make([]byte, 32))
	}

	builder, err := sui.NewTxBuilder(c.Asset.GetChain().Base())
	if err != nil {
		return input, fmt.Errorf("could not create tx builder: %w", err)
	}

	tx, err := builder.Transfer(args, input)
	if err != nil {
		return input, fmt.Errorf("could not build tx: %w", err)
	}

	gasFee, ok, err := c.simulateTransactionGasFee(ctx, tx, isNative)
	if err != nil {
		return nil, fmt.Errorf("failed to get transaction gas fee: %w", err)
	}
	if !ok && input.AddressBalance != nil {
		return nil, fmt.Errorf("Sui simulation failed for address-balance transaction")
	}
	if ok {
		input.GasBudget = gasFee
		if input.AddressBalance != nil {
			_, err = builder.Transfer(args, input)
			if err != nil {
				return nil, fmt.Errorf("insufficient funds for estimated gas: %w", err)
			}
			if input.GasBudget < gasFee {
				return nil, fmt.Errorf("insufficient SUI in gas funding source: need %d, have %d", gasFee, input.GasBudget)
			}
		}
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
