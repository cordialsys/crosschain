package commands

import (
	"fmt"

	xc "github.com/cordialsys/crosschain"
	xcaddress "github.com/cordialsys/crosschain/address"
	"github.com/cordialsys/crosschain/cmd/xc/setup"
	"github.com/cordialsys/crosschain/config"
	"github.com/cordialsys/crosschain/factory/signer"
	"github.com/spf13/cobra"
)

func CmdAddress() *cobra.Command {
	var privateKeyRef string
	var format string
	cmd := &cobra.Command{
		Use:   "address",
		Short: fmt.Sprintf("Derive an address from the %s environment variable.", signer.EnvPrivateKey),
		Args:  cobra.ExactArgs(0),
		RunE: func(cmd *cobra.Command, args []string) error {
			xcFactory := setup.UnwrapXc(cmd.Context())
			chainConfig := setup.UnwrapChain(cmd.Context())
			algorithm, _ := cmd.Flags().GetString("algorithm")
			addressArgs := []xcaddress.AddressOption{}
			if algorithm != "" {
				addressArgs = append(addressArgs, xcaddress.OptionAlgorithm(xc.SignatureType(algorithm)))
			}
			if format != "" {
				addressArgs = append(addressArgs, xcaddress.OptionFormat(xc.AddressFormat(format)))
			}

			privateKeyInput, err := config.GetSecret(privateKeyRef)
			if err != nil {
				return fmt.Errorf("could not get secret: %v", err)
			}
			if privateKeyInput == "" {
				return fmt.Errorf("secret reference (default env:%s) loaded empty value", privateKeyRef)
			}

			signer, err := xcFactory.NewSigner(chainConfig.Base(), privateKeyInput, addressArgs...)
			if err != nil {
				return fmt.Errorf("could not import private key: %v", err)
			}

			publicKey, err := signer.PublicKey()
			if err != nil {
				return fmt.Errorf("could not create public key: %v", err)
			}

			addressBuilder, err := xcFactory.NewAddressBuilder(chainConfig.Base(), addressArgs...)
			if err != nil {
				return fmt.Errorf("could not create address builder: %v", err)
			}

			from, err := addressBuilder.GetAddressFromPublicKey(publicKey)
			if err != nil {
				return fmt.Errorf("could not derive address: %v", err)
			}

			fmt.Println(from)

			return nil
		},
	}
	cmd.Flags().StringVar(&privateKeyRef, "key", "env:"+signer.EnvPrivateKey, "Private key reference")
	cmd.Flags().String("contract", "", "Contract address of asset to send, if applicable")
	cmd.Flags().StringVar(&format, "format", "", "Format of the address")
	return cmd
}
