package kiln

import (
	"context"
	"fmt"

	xc "github.com/cordialsys/crosschain"
	xcbuilder "github.com/cordialsys/crosschain/builder"
	"github.com/cordialsys/crosschain/builder/validation"
	"github.com/cordialsys/crosschain/chain/evm/address"
	"github.com/cordialsys/crosschain/chain/evm/builder"
	evmclient "github.com/cordialsys/crosschain/chain/evm/client"
	"github.com/cordialsys/crosschain/chain/evm/tx"
	"github.com/cordialsys/crosschain/chain/evm/tx_input"
	xcclient "github.com/cordialsys/crosschain/client"
	"github.com/cordialsys/crosschain/client/services"
	"github.com/cordialsys/crosschain/client/services/kiln"
	"github.com/sirupsen/logrus"
)

type Client struct {
	rpcClient  *evmclient.Client
	kilnClient *kiln.Client
	chain      *xc.ChainConfig
}

var _ xcclient.StakingClient = &Client{}

func toStakingState(status string) (xcclient.StakeState, bool) {
	// ethereum validator states
	state, ok := evmclient.ValidatorStatus(status).ToState()
	if ok {
		return state, true
	}
	// kiln-specific states
	switch status {
	case "deposit_in_progress":
		state = xcclient.Activating
	case "active_ongoing":
		state = xcclient.Active
	case "unstaked":
		// this means the eth has been returned
	}

	return state, state != ""
}

func NewClient(rpcClient *evmclient.Client, chain *xc.ChainConfig, kilnCfg *services.KilnConfig) (xcclient.StakingClient, error) {
	// rpcClient, err := evmclient.NewClient(chain)
	// if err != nil {
	// 	return nil, err
	// }
	apiToken, err := kilnCfg.ApiToken.Load()
	if err != nil {
		return nil, err
	}
	kilnClient, err := kiln.NewClient(string(chain.Chain), kilnCfg.BaseUrl, apiToken)
	if err != nil {
		return nil, err
	}
	return &Client{rpcClient, kilnClient, chain}, nil
}

func (cli *Client) FetchStakeBalance(ctx context.Context, args xcclient.StakedBalanceArgs) ([]*xcclient.StakedBalance, error) {
	// On evm stakes are identified solely by validator, so we can map to either validator or account ID
	validator, ok := args.GetValidator()
	if !ok {
		return nil, fmt.Errorf("must provider a validator to lookup balance for")
	}

	// RPC is the most reliable place to get information on the stake
	validatorBal, err := cli.rpcClient.FetchStakeBalance(ctx, args)
	if err != nil {
		logrus.WithError(err).Debug("could not locate validator")
	} else {
		return validatorBal, nil
	}

	// However, it's not available via RPC during the first 'activating' period,
	// so we rely on Kiln instead.
	res, err := cli.kilnClient.GetStakesByValidator(validator)
	if err != nil {
		return nil, err
	}
	if len(res.Data) == 0 {
		return nil, nil
	}
	if res.Data[0].State == "unstaked" {
		// this means the eth has been sent back, so no balance is in a staking state.
		return []*xcclient.StakedBalance{}, nil
	}
	// Assume it's always 32 ETH until we can read the stake from RPC
	bal, _ := xc.NewAmountHumanReadableFromStr("32")
	amount := bal.ToBlockchain(18)
	status, ok := toStakingState(res.Data[0].State)
	if !ok {
		// assume it's still activating
		status = xcclient.Activating
		logrus.WithField("kiln-state", res.Data[0].State).Warn("unknown validator state")
	}
	return []*xcclient.StakedBalance{
		xcclient.NewStakedBalance(amount, status, validator, ""),
	}, nil
}

func (cli *Client) FetchStakingInput(ctx context.Context, args xcbuilder.StakeArgs) (xc.StakeTxInput, error) {
	stakingInput, err := cli.FetchKilnInput(ctx, args)
	if err != nil {
		return nil, err
	}

	partialTxInput, err := cli.rpcClient.FetchUnsimulatedInput(ctx, args.GetFrom(), "")
	if err != nil {
		return nil, err
	}
	stakingInput.TxInput = *partialTxInput

	builder, err := builder.NewTxBuilder(cli.chain.Base())
	if err != nil {
		return nil, fmt.Errorf("could not prepare to simulate: %v", err)
	}

	exampleTf, err := builder.Stake(args, stakingInput)
	if err != nil {
		return nil, fmt.Errorf("could not prepare to simulate: %v", err)
	}

	gasLimit, err := cli.rpcClient.SimulateGasWithLimit(ctx, args.GetFrom(), exampleTf.(*tx.Tx))
	if err != nil {
		return nil, err
	}
	stakingInput.GasLimit = gasLimit

	return stakingInput, nil
}

func (cli *Client) FetchKilnInput(ctx context.Context, args xcbuilder.StakeArgs) (*tx_input.BatchDepositInput, error) {
	count, err := validation.Count32EthChunks(args.GetAmount())
	if err != nil {
		return nil, err
	}
	accountId, _ := args.GetStakeAccount()
	acc, err := cli.kilnClient.ResolveAccount(accountId)
	if err != nil {
		return nil, err
	}

	keys, err := cli.kilnClient.CreateValidatorKeys(acc.ID, string(args.GetFrom()), int(count))
	if err != nil {
		return nil, fmt.Errorf("could not create validator keys: %v", err)
	}

	input := tx_input.NewBatchDepositInput()
	pubkeys := []string{}
	sigs := []string{}

	// tolerate ambiguous kiln type
	if keys.Response1 != nil {
		for _, data := range keys.Response1.Data {
			pubkeys = append(pubkeys, data.PubKey)
			sigs = append(sigs, data.Signature)
		}
	} else if keys.Response2 != nil {
		pubkeys = append(pubkeys, keys.Response2.Data.PubKeys...)
		sigs = append(sigs, keys.Response2.Data.Signatures...)
	}
	for _, pubkey := range pubkeys {
		pubkeyBz, err := address.DecodeHex(pubkey)
		if err != nil {
			return nil, fmt.Errorf("kiln provided invalid validator public key %s: %v", pubkey, err)
		}
		input.PublicKeys = append(input.PublicKeys, pubkeyBz)
	}
	for _, sig := range sigs {
		sigBiz, err := address.DecodeHex(sig)
		if err != nil {
			return nil, fmt.Errorf("kiln provided invalid signature %s: %v", sig, err)
		}
		input.Signatures = append(input.Signatures, sigBiz)
	}
	return input, nil
}

func (cli *Client) FetchUnstakingInput(ctx context.Context, args xcbuilder.StakeArgs) (xc.UnstakeTxInput, error) {

	validatorInput, ok := args.GetValidator()
	var activeValidators [][]byte
	if ok {
		stakes, err := cli.kilnClient.GetStakesByValidator(validatorInput)
		if err != nil || len(stakes.Data) == 0 {
			logrus.WithError(err).Debug("could not locate validator with kiln")
		}
		bz, err := address.DecodeHex(validatorInput)
		if err != nil {
			return nil, fmt.Errorf("invalid validator public key %s: %v", validatorInput, err)
		}
		activeValidators = [][]byte{bz}
	} else {
		var err error
		activeValidators, err = cli.FetchActiveKilnValidators(ctx, args.GetFrom())
		if err != nil {
			return nil, err
		}
	}

	partialTxInput, err := cli.rpcClient.FetchUnsimulatedInput(ctx, args.GetFrom(), "")
	if err != nil {
		return nil, err
	}
	stakingInput := &tx_input.ExitRequestInput{
		TxInput:    *partialTxInput,
		PublicKeys: activeValidators,
	}

	builder, err := builder.NewTxBuilder(cli.chain.Base())
	if err != nil {
		return nil, fmt.Errorf("could not prepare to simulate: %v", err)
	}

	exampleTf, err := builder.Unstake(args, stakingInput)
	if err != nil {
		return nil, fmt.Errorf("could not prepare to simulate: %v", err)
	}

	gasLimit, err := cli.rpcClient.SimulateGasWithLimit(ctx, args.GetFrom(), exampleTf.(*tx.Tx))
	if err != nil {
		return nil, err
	}
	stakingInput.GasLimit = gasLimit

	return stakingInput, nil
}

func (cli *Client) FetchActiveKilnValidators(ctx context.Context, from xc.Address) ([][]byte, error) {
	stakes, err := cli.kilnClient.GetAllStakesByOwner(string(from))
	if err != nil {
		return nil, fmt.Errorf("could not fetch validators: %v", err)
	}

	pubkeys := []string{}
	pubkeysBz := [][]byte{}

	for _, stake := range stakes {
		status := ""
		if stake.State == "active_ongoing" && stake.IsKiln {
			// double check it hasn't had an exit requested yet via RPC
			validator, err := cli.rpcClient.FetchValidator(ctx, stake.ValidatorAddress)
			if err != nil {
				return nil, fmt.Errorf("could not lookup validator: %v", err)
			}
			state, _ := validator.Data.Status.ToState()

			if state != xcclient.Deactivating && state != xcclient.Inactive {
				pubkeys = append(pubkeys, stake.ValidatorAddress)
			}
		}
		logrus.WithFields(logrus.Fields{
			"kiln":   stake.IsKiln,
			"pubkey": stake.ValidatorAddress,
			"status": status,
		}).Debug("validator")
	}

	for _, pubkey := range pubkeys {
		pubkeyBz, err := address.DecodeHex(pubkey)
		if err != nil {
			return nil, fmt.Errorf("kiln provided invalid validator public key %s: %v", pubkey, err)
		}
		pubkeysBz = append(pubkeysBz, pubkeyBz)
	}

	return pubkeysBz, nil
}

func (cli *Client) FetchWithdrawInput(ctx context.Context, args xcbuilder.StakeArgs) (xc.WithdrawTxInput, error) {
	return nil, fmt.Errorf("ethereum stakes are withdrawn automatically")
}
