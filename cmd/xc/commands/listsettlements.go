package commands

import (
	"context"
	"fmt"

	xc "github.com/cordialsys/crosschain"
	"github.com/cordialsys/crosschain/client"
	"github.com/cordialsys/crosschain/cmd/xc/setup"
	"github.com/cordialsys/crosschain/factory/signer"
	"github.com/spf13/cobra"
)

func CmdListSettlements() *cobra.Command {
	var contract string
	var privateKeyRef string
	var format string
	cmd := &cobra.Command{
		Use:   "list-settlements [address]",
		Short: "List accepted offers awaiting settlement that are visible to an address.",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			xcFactory := setup.UnwrapXc(cmd.Context())
			chainConfig := setup.UnwrapChain(cmd.Context())

			address, err := inputAddressOrDerived(xcFactory, chainConfig, args, privateKeyRef, format)
			if err != nil {
				return err
			}

			rpcClient, err := xcFactory.NewClient(chainConfig)
			if err != nil {
				return err
			}
			offerClient, ok := rpcClient.(client.OfferClient)
			if !ok {
				return fmt.Errorf("chain %s does not support listing settlements", chainConfig.Chain)
			}

			options := []client.GetOfferOption{}
			if contract != "" {
				options = append(options, client.OfferOptionContract(xc.ContractAddress(contract)))
			}
			settlements, err := offerClient.ListSettlements(context.Background(), client.NewOfferArgs(address, options...))
			if err != nil {
				return fmt.Errorf("could not list settlements for address %s: %v", address, err)
			}

			fmt.Println(asJson(settlements))
			return nil
		},
	}
	cmd.Flags().StringVar(&privateKeyRef, "key", "env:"+signer.EnvPrivateKey, "Private key reference")
	cmd.Flags().StringVar(&contract, "contract", "", "Optional contract of token asset")
	cmd.Flags().StringVar(&format, "format", "", "Optional address format for chains that use multiple address formats")
	return cmd
}
