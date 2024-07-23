package kiln

import (
	"context"
	"fmt"

	xc "github.com/cordialsys/crosschain"
	xcbuilder "github.com/cordialsys/crosschain/builder"
	"github.com/cordialsys/crosschain/chain/evm/address"
	"github.com/cordialsys/crosschain/chain/evm/builder"
	evmclient "github.com/cordialsys/crosschain/chain/evm/client"
	"github.com/cordialsys/crosschain/chain/evm/tx"
	"github.com/cordialsys/crosschain/chain/evm/tx_input"
	"github.com/cordialsys/crosschain/client/staking"
	"github.com/cordialsys/crosschain/examples/staking/kiln/api"
	"github.com/sirupsen/logrus"
)

type ClientParams struct {
	// accountId string
}

// func (params *ClientParams) SetAccountID(id string) {
// 	params.accountId = id
// }

type Client struct {
	ClientParams
	rpcClient  *evmclient.Client
	kilnClient *api.Client
	chain      *xc.ChainConfig
}

var _ staking.StakingClient = &Client{}

func toStakingState(status string) (staking.State, bool) {
	var state staking.State = ""
	switch status {
	case "pending_initialized", "deposit_in_progress":
		state = staking.Activating
	case "active_ongoing":
		state = staking.Activated
	case "withdrawal_possible", "withdrawal_done", "exited_unslashed", "exited_slashed":
		state = staking.Inactive
	case "active_exiting", "pending_queued":
		state = staking.Deactivating
	default:
	}
	return state, state != ""
}

func NewClient(rpcClient *evmclient.Client, chain *xc.ChainConfig, stakingCfg *staking.StakingConfig) (staking.StakingClient, error) {
	// rpcClient, err := evmclient.NewClient(chain)
	// if err != nil {
	// 	return nil, err
	// }
	apiToken, err := stakingCfg.Kiln.ApiToken.Load()
	if err != nil {
		return nil, err
	}
	kilnClient, err := api.NewClient(string(chain.Chain), stakingCfg.Kiln.BaseUrl, apiToken)
	if err != nil {
		return nil, err
	}
	params := ClientParams{}
	return &Client{params, rpcClient, kilnClient, chain}, nil
}

func (cli *Client) FetchStakeBalance(ctx context.Context, address xc.Address, validator string, stakeAccount xc.Address) ([]*staking.Balance, error) {
	// On evm stakes are identified solely by validator, so we can map to either validator or account ID
	if validator == "" && stakeAccount != "" {
		validator = string(stakeAccount)
	}
	// Assume it's always 32 ETH until we can read the stake from RPC
	bal, _ := xc.NewAmountHumanReadableFromStr("32")
	amount := bal.ToBlockchain(18)

	status := staking.Activating
	// RPC is the most reliable place to get information on the stake
	val, err := cli.rpcClient.FetchValidator(ctx, validator)
	if err != nil {
		logrus.WithError(err).Debug("could not locate validator")
	} else {
		gwei, _ := xc.NewAmountHumanReadableFromStr(val.Data.Validator.EffectiveBalance)
		amount = gwei.ToBlockchain(9)
		var ok bool
		status, ok = toStakingState(val.Data.Status)
		if !ok {
			status = staking.Activating
			logrus.Warn("unknown beacon validator state", status)
		}
		return []*staking.Balance{
			{
				State:  status,
				Amount: amount,
			},
		}, nil
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
	var ok bool
	status, ok = toStakingState(val.Data.Status)
	if !ok {
		status = staking.Activating
		logrus.Warn("unknown validator state", status)
	}
	return []*staking.Balance{
		{
			State:  status,
			Amount: amount,
		},
	}, nil
}

func (cli *Client) FetchStakingInput(ctx context.Context, args xcbuilder.StakeArgs) (xc.StakingInput, error) {
	stakingInput, err := cli.FetchKilnInput(ctx, args)
	if err != nil {
		return nil, err
	}

	partialTxInput, err := cli.rpcClient.FetchUnsimulatedInput(ctx, args.GetFrom())
	if err != nil {
		return nil, err
	}
	stakingInput.TxInput = *partialTxInput

	builder, err := builder.NewTxBuilder(cli.chain)
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

func (cli *Client) FetchKilnInput(ctx context.Context, args xcbuilder.StakeArgs) (*tx_input.MultiDepositInput, error) {
	count, err := tx_input.DivideAmount(cli.chain, args.GetAmount())
	if err != nil {
		return nil, err
	}
	accountId, _ := args.GetAccountId()
	acc, err := cli.kilnClient.ResolveAccount(accountId)
	if err != nil {
		return nil, err
	}

	keys, err := cli.kilnClient.CreateValidatorKeys(acc.ID, string(args.GetFrom()), int(count))
	if err != nil {
		return nil, fmt.Errorf("could not create validator keys: %v", err)
	}

	input := &tx_input.MultiDepositInput{
		StakingInputEnvelope: tx_input.NewMultidepositStakingInput().StakingInputEnvelope,
	}
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

func (cli *Client) FetchUnstakingInput(ctx context.Context, args xcbuilder.StakeArgs) (xc.StakingInput, error) {
	stakingInput, err := cli.FetchKilnUnstakeInput(ctx, args)
	if err != nil {
		return nil, err
	}

	partialTxInput, err := cli.rpcClient.FetchUnsimulatedInput(ctx, args.GetFrom())
	if err != nil {
		return nil, err
	}
	stakingInput.TxInput = *partialTxInput

	builder, err := builder.NewTxBuilder(cli.chain)
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

func (cli *Client) FetchKilnUnstakeInput(ctx context.Context, args xcbuilder.StakeArgs) (*tx_input.MultiDepositInput, error) {
	stakes, err := cli.kilnClient.GetAllStakesByOwner(string(args.GetFrom()))
	if err != nil {
		return nil, fmt.Errorf("could not fetch validators: %v", err)
	}

	input := &tx_input.MultiDepositInput{
		StakingInputEnvelope: tx_input.NewMultidepositStakingInput().StakingInputEnvelope,
	}
	pubkeys := []string{}

	for _, stake := range stakes {
		status := ""
		if stake.State == "active_ongoing" && stake.IsKiln {
			// double check it hasn't had an exit requested yet via RPC
			validator, err := cli.rpcClient.FetchValidator(ctx, stake.ValidatorAddress)
			if err != nil {
				return nil, fmt.Errorf("could not lookup validator: %v", err)
			}
			status = validator.Data.Status
			state, _ := toStakingState(status)

			if state != staking.Deactivating && state != staking.Inactive {
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
		input.PublicKeys = append(input.PublicKeys, pubkeyBz)
	}

	return input, nil
}
