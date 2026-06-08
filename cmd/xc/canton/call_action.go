package canton

import (
	"encoding/json"
	"fmt"
	"time"

	xc "github.com/cordialsys/crosschain"
	"github.com/cordialsys/crosschain/builder"
	xccall "github.com/cordialsys/crosschain/call"
	"github.com/cordialsys/crosschain/client"
	"github.com/cordialsys/crosschain/cmd/xc/commands"
	"github.com/cordialsys/crosschain/cmd/xc/setup"
	"github.com/cordialsys/crosschain/config"
	"github.com/cordialsys/crosschain/factory"
	"github.com/cordialsys/crosschain/factory/drivers"
	"github.com/cordialsys/crosschain/factory/signer"
	fsigner "github.com/cordialsys/crosschain/factory/signer"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

func CmdOfferAccept() *cobra.Command {
	return newCallActionCommand(
		"offer-accept <contract-id> [address]",
		"Accept a pending Canton offer by contract id.",
		xccall.OfferAccept,
	)
}

func CmdSettlementComplete() *cobra.Command {
	return newCallActionCommand(
		"settlement-complete <contract-id> [address]",
		"Complete a pending Canton settlement by contract id.",
		xccall.SettlementComplete,
	)
}

func newCallActionCommand(use string, short string, method xccall.Method) *cobra.Command {
	var privateKeyRef string
	var timeout time.Duration

	cmd := &cobra.Command{
		Use:   use,
		Short: short,
		Args:  cobra.RangeArgs(1, 2),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			xcFactory := setup.UnwrapXc(ctx)
			chainConfig := setup.UnwrapChain(ctx)
			if chainConfig.Driver != xc.DriverCanton {
				return fmt.Errorf("canton %s requires --chain CANTON, got driver %q", cmd.Name(), chainConfig.Driver)
			}

			contractID := args[0]
			xcSigner, address, err := signerAndAddressFromArgs(xcFactory, chainConfig, args[1:], privateKeyRef)
			if err != nil {
				return err
			}

			signerCollection := fsigner.NewCollection()
			signerCollection.AddMainSigner(xcSigner, address)

			rpcClient, err := xcFactory.NewClient(chainConfig)
			if err != nil {
				return fmt.Errorf("could not load client: %v", err)
			}
			callClient, ok := rpcClient.(client.CallClient)
			if !ok {
				return fmt.Errorf("chain %s does not support call actions", chainConfig.Chain)
			}

			payload, err := json.Marshal(xccall.SomeContractCall{ContractID: contractID})
			if err != nil {
				return err
			}
			callTx, err := drivers.NewCall(chainConfig.Base(), method, payload, []xc.Address{address})
			if err != nil {
				return fmt.Errorf("could not build call transaction: %v", err)
			}

			callArgs, err := builder.NewCallArgs(chainConfig.Base())
			if err != nil {
				return fmt.Errorf("could not build call arguments: %v", err)
			}

			input, err := callClient.FetchCallInput(ctx, callTx, callArgs)
			if err != nil {
				return fmt.Errorf("could not fetch call input: %v", err)
			}

			tx, err := commands.PrepareCallForSubmit(callTx, input, signerCollection)
			if err != nil {
				return fmt.Errorf("could not prepare call transaction: %v", err)
			}

			if err := commands.SubmitTransaction(chainConfig.Chain, rpcClient, tx, timeout); err != nil {
				return fmt.Errorf("could not submit call transaction: %v", err)
			}

			logrus.WithField("hash", tx.Hash()).Info("submitted tx")
			fmt.Println(asJSON(map[string]any{
				"hash":       tx.Hash(),
				"method":     method,
				"address":    address,
				"contractId": contractID,
			}))
			return nil
		},
	}

	cmd.Flags().StringVar(&privateKeyRef, "key", "env:"+signer.EnvPrivateKey, "Private key reference")
	cmd.Flags().DurationVar(&timeout, "timeout", 1*time.Minute, "Amount of time to wait for transaction submission.")
	return cmd
}

func signerAndAddressFromArgs(xcFactory *factory.Factory, chainConfig *xc.ChainConfig, args []string, keyRef string) (*signer.Signer, xc.Address, error) {
	if len(args) == 0 {
		return commands.SignerAndAddress(xcFactory, chainConfig, keyRef)
	}

	privateKeyInput, err := config.GetSecret(keyRef)
	if err != nil {
		return nil, "", fmt.Errorf("could not get secret: %v", err)
	}
	if privateKeyInput == "" {
		return nil, "", fmt.Errorf("must set env %s", signer.EnvPrivateKey)
	}
	xcSigner, err := xcFactory.NewSigner(chainConfig.Base(), privateKeyInput)
	if err != nil {
		return nil, "", fmt.Errorf("could not import private key: %v", err)
	}
	return xcSigner, xc.Address(args[0]), nil
}

func asJSON(data any) string {
	bz, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		panic(err)
	}
	return string(bz)
}
