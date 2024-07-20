package main

import (
	"context"
	"fmt"

	xc "github.com/cordialsys/crosschain"
	"github.com/cordialsys/crosschain/chain/evm/address"
	evmclient "github.com/cordialsys/crosschain/chain/evm/client"
	"github.com/cordialsys/crosschain/chain/evm/tx_input"
	"github.com/cordialsys/crosschain/client/staking"
	"github.com/cordialsys/crosschain/examples/staking/kiln/api"
	"github.com/sirupsen/logrus"
)

type ClientParams struct {
	accountId string
}

func (params *ClientParams) SetAccountID(id string) {
	params.accountId = id
}

type Client struct {
	ClientParams
	rpcClient  *evmclient.Client
	kilnClient *api.Client
	chain      *xc.ChainConfig
}

var _ staking.StakingClient = &Client{}

func NewClient(chain *xc.ChainConfig, variant xc.StakingVariant, url string, apiKey string) (staking.StakingClient, error) {
	rpcClient, err := evmclient.NewClient(chain)
	if err != nil {
		return nil, err
	}
	kilnClient, err := api.NewClient(string(chain.Chain), url, apiKey)
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
		switch val.Data.Status {
		case "pending_initialized":
			status = staking.Activating
		case "active_ongoing":
			status = staking.Activated
		case "withdrawal_possible", "withdrawal_done", "exited_unslashed", "exited_slashed":
			status = staking.Inactive
		case "active_exiting", "pending_queued":
			status = staking.Deactivating
		default:
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
	res, err := cli.kilnClient.GetStakes(validator)
	if err != nil {
		return nil, err
	}
	if len(res.Data) == 0 {
		return nil, nil
	}
	switch res.Data[0].State {
	case "deposit_in_progress":
		status = staking.Activating
	case "active_ongoing":
		status = staking.Activated
	default:
		logrus.Warn("unknown kiln state", status)
	}
	return []*staking.Balance{
		{
			State:  status,
			Amount: amount,
		},
	}, nil
}

func (cli *Client) FetchStakeInput(ctx context.Context, addr xc.Address, validator string, amount xc.AmountBlockchain) (xc.StakingInput, error) {
	count, err := tx_input.DivideAmount(cli.chain, amount)
	if err != nil {
		return nil, err
	}
	acc, err := cli.kilnClient.ResolveAccount(cli.accountId)
	if err != nil {
		return nil, err
	}

	keys, err := cli.kilnClient.CreateValidatorKeys(acc.ID, string(addr), int(count))
	if err != nil {
		return nil, fmt.Errorf("could not create validator keys: %v", err)
	}

	input := &tx_input.KilnStakingInput{
		StakingInputEnvelope: tx_input.NewKilnStakingInput().StakingInputEnvelope,
		// trusted input, to be set later
		ContractAddress: "",
		// trusted input, to be set later
		Credentials: nil,
		// TODO shouldn't need to set this here
		Amount: amount,
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
