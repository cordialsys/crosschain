package figment

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
	xcclient "github.com/cordialsys/crosschain/client"
	"github.com/cordialsys/crosschain/client/services"
	"github.com/cordialsys/crosschain/client/services/figment"
	testutil "github.com/cordialsys/crosschain/testutil/types"
	"github.com/sirupsen/logrus"
)

type Client struct {
	rpcClient      *evmclient.Client
	providerClient *figment.Client
	chain          *xc.ChainConfig
}

var _ xcclient.StakingClient = &Client{}
var _ xcclient.ManualUnstakingClient = &Client{}

func toStakingState(status string) (xcclient.State, bool) {
	// ethereum validator states
	state, ok := evmclient.ValidatorStatus(status).ToState()
	if ok {
		return state, true
	}
	// provider-specific states
	switch status {
	case "deposited_not_finalized", "pending_queued":
		state = xcclient.Activating
	case "active_ongoing":
		state = xcclient.Active
	case "unstaked", "withdrawal_done":
		// this means the eth has been returned
	}

	return state, state != ""
}

func NewClient(rpcClient *evmclient.Client, chain *xc.ChainConfig, figmentCfg *services.FigmentConfig) (xcclient.StakingClient, error) {
	apiToken, err := figmentCfg.ApiToken.Load()
	if err != nil {
		return nil, err
	}
	kilnClient, err := figment.NewClient(string(chain.Chain), figmentCfg.Network, figmentCfg.BaseUrl, apiToken)
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
	validatorBal, err := cli.rpcClient.FetchValidatorBalance(ctx, validator)
	if err != nil {
		logrus.WithError(err).Debug("could not locate validator")
	} else {
		return []*xcclient.StakedBalance{validatorBal}, nil
	}

	logrus.WithError(err).Debug("validator not found via ethereum beacon RPC")

	// Lookup using figment API
	res, err := cli.providerClient.GetValidator(validator)
	if err != nil {
		return nil, err
	}
	bal, _ := xc.NewAmountHumanReadableFromStr("32")
	amount := bal.ToBlockchain(18)

	state, ok := toStakingState(res.Data.Status)
	if !ok {
		// assume it's still activating
		state = xcclient.Activating
		logrus.WithField("figment-state", res.Data.Status).Warn("unknown validator state")
	}
	return []*xcclient.StakedBalance{
		xcclient.NewStakedBalance(amount, state, validator, ""),
	}, nil

}

func (cli *Client) FetchStakingInput(ctx context.Context, args xcbuilder.StakeArgs) (xc.StakeTxInput, error) {
	partialTxInput, err := cli.rpcClient.FetchUnsimulatedInput(ctx, args.GetFrom())
	if err != nil {
		return nil, err
	}
	_ = partialTxInput
	count, err := tx_input.Count32EthChunks(cli.chain, args.GetAmount())
	if err != nil {
		return nil, err
	}
	res, err := cli.providerClient.CreateValidator(int(count), string(args.GetFrom()))
	if err != nil {
		return nil, err
	}
	testutil.JsonPrint(res)
	stakingInput := tx_input.NewBatchDepositInput()
	stakingInput.TxInput = *partialTxInput

	for _, validator := range res.Data {
		pubkeyBz, err := address.DecodeHex(validator.Pubkey)
		if err != nil {
			return nil, fmt.Errorf("failed to decode figment validator pubkey: %v", err)
		}
		signatureBz, err := address.DecodeHex(validator.DepositData.Signature)
		if err != nil {
			return nil, fmt.Errorf("failed to decode figment validator signature: %v", err)
		}
		stakingInput.PublicKeys = append(stakingInput.PublicKeys, pubkeyBz)
		stakingInput.Signatures = append(stakingInput.Signatures, signatureBz)
	}

	builder, err := builder.NewTxBuilder(cli.chain)
	if err != nil {
		return nil, fmt.Errorf("could not prepare to simulate: %v", err)
	}
	exampleTx, err := builder.Stake(args, stakingInput)
	if err != nil {
		return nil, fmt.Errorf("could not prepare to simulate: %v", err)
	}
	gasLimit, err := cli.rpcClient.SimulateGasWithLimit(ctx, args.GetFrom(), exampleTx.(*tx.Tx))
	if err != nil {
		return nil, err
	}
	stakingInput.GasLimit = gasLimit

	return stakingInput, nil
}

func (cli *Client) FetchUnstakingInput(ctx context.Context, args xcbuilder.StakeArgs) (xc.UnstakeTxInput, error) {
	return nil, fmt.Errorf("ethereum stakes by Figment are exited on demand using API or by manual request to Figment")
}

func (cli *Client) FetchWithdrawInput(ctx context.Context, args xcbuilder.StakeArgs) (xc.WithdrawTxInput, error) {
	return nil, fmt.Errorf("ethereum stakes are withdrawn automatically by the protocol")
}

func (cli *Client) InitiateManualUnstaking(ctx context.Context, args xcbuilder.StakeArgs) error {
	count, err := tx_input.Count32EthChunks(cli.chain, args.GetAmount())
	if err != nil {
		return err
	}
	_, err = cli.providerClient.ExitValidators(string(args.GetFrom()), int(count))
	return err
}
