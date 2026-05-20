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

func CmdListOffers() *cobra.Command {
	var contract string
	var privateKeyRef string
	var format string
	cmd := &cobra.Command{
		Use:   "list-offers [address]",
		Short: "List pending offers visible to an address.",
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
				return fmt.Errorf("chain %s does not support listing offers", chainConfig.Chain)
			}

			options := []client.GetOfferOption{}
			if contract != "" {
				options = append(options, client.OfferOptionContract(xc.ContractAddress(contract)))
			}
			offers, err := offerClient.ListPendingOffers(context.Background(), client.NewOfferArgs(address, options...))
			if err != nil {
				return fmt.Errorf("could not list offers for address %s: %v", address, err)
			}

			fmt.Println(asJson(offers))
			return nil
		},
	}
	cmd.Flags().StringVar(&privateKeyRef, "key", "env:"+signer.EnvPrivateKey, "Private key reference")
	cmd.Flags().StringVar(&contract, "contract", "", "Optional contract of token asset")
	cmd.Flags().StringVar(&format, "format", "", "Optional address format for chains that use multiple address formats")
	return cmd
}
