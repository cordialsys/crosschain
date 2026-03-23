package commands

import (
	"context"
	"encoding/hex"
	"fmt"

	xcbuilder "github.com/cordialsys/crosschain/builder"
	"github.com/cordialsys/crosschain/chain/canton/tx_input"
	xclient "github.com/cordialsys/crosschain/client"
	"github.com/cordialsys/crosschain/cmd/xc/setup"
	"github.com/cordialsys/crosschain/config"
	"github.com/cordialsys/crosschain/factory/signer"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

func CmdCreateAccount() *cobra.Command {
	var privateKeyRef string

	cmd := &cobra.Command{
		Use:   "create-account",
		Short: "Advance account registration and return the next signature request when one is needed.",
		Args:  cobra.ExactArgs(0),
		RunE: func(cmd *cobra.Command, args []string) error {
			xcFactory := setup.UnwrapXc(cmd.Context())
			chainConfig := setup.UnwrapChain(cmd.Context())

			privateKeyInput, err := config.GetSecret(privateKeyRef)
			if err != nil {
				return fmt.Errorf("could not get secret: %v", err)
			}
			if privateKeyInput == "" {
				return fmt.Errorf("must set env %s", signer.EnvPrivateKey)
			}

			xcSigner, err := xcFactory.NewSigner(chainConfig.Base(), privateKeyInput)
			if err != nil {
				return fmt.Errorf("could not import private key: %v", err)
			}
			publicKey, err := xcSigner.PublicKey()
			if err != nil {
				return fmt.Errorf("could not derive public key: %v", err)
			}
			addressBuilder, err := xcFactory.NewAddressBuilder(chainConfig.Base())
			if err != nil {
				return fmt.Errorf("could not create address builder: %v", err)
			}
			address, err := addressBuilder.GetAddressFromPublicKey(publicKey)
			if err != nil {
				return fmt.Errorf("could not derive address: %v", err)
			}

			logrus.WithField("address", address).Info("registering account")

			rpcClient, err := xcFactory.NewClient(chainConfig)
			if err != nil {
				return fmt.Errorf("could not load client: %v", err)
			}
			accountClient, ok := rpcClient.(xclient.CreateAccountClient)
			if !ok {
				return fmt.Errorf("chain %s does not support account creation", chainConfig.Chain)
			}
			txBuilder, err := xcFactory.NewTxBuilder(chainConfig.Base())
			if err != nil {
				return fmt.Errorf("could not create tx builder: %v", err)
			}
			accountBuilder, ok := txBuilder.(xcbuilder.AccountCreation)
			if !ok {
				return fmt.Errorf("chain %s does not support create-account transactions", chainConfig.Chain)
			}

			createArgs := xclient.NewCreateAccountArgs(address, publicKey)
			input, err := accountClient.FetchCreateAccountInput(context.Background(), createArgs)
			if err != nil {
				return fmt.Errorf("could not fetch create-account input: %v", err)
			}
			if input == nil {
				fmt.Println(asJson(map[string]string{
					"address": string(address),
					"chain":   string(chainConfig.Chain),
					"status":  "registered",
				}))
				return nil
			}

			cantonInput, ok := input.(*tx_input.CreateAccountInput)
			if !ok {
				return fmt.Errorf("invalid create-account input type: %T", input)
			}
			if err := cantonInput.VerifySignaturePayloads(); err != nil {
				return fmt.Errorf("hash verification failed: %v", err)
			}
			builderArgs, err := xcbuilder.NewCreateAccountArgs(chainConfig.Chain, address, publicKey)
			if err != nil {
				return fmt.Errorf("could not build create-account args: %v", err)
			}
			tx, err := accountBuilder.CreateAccount(builderArgs, input)
			if err != nil {
				return fmt.Errorf("could not build create-account tx: %v", err)
			}

			sighashes, err := tx.Sighashes()
			if err != nil {
				return fmt.Errorf("could not get sighashes: %v", err)
			}
			if len(sighashes) != 1 {
				return fmt.Errorf("expected exactly 1 signature request, got %d", len(sighashes))
			}

			logrus.WithField("count", len(sighashes)).Info("signature required")
			for i, sh := range sighashes {
				logrus.WithField("index", i).WithField("payload", hex.EncodeToString(sh.Payload)).Debug("signature request")
			}

			serializedInput, err := tx.Serialize()
			if err != nil {
				return fmt.Errorf("could not serialize create-account tx: %v", err)
			}

			fmt.Println(asJson(map[string]any{
				"address":     string(address),
				"chain":       string(chainConfig.Chain),
				"status":      "signature_required",
				"stage":       cantonInput.Stage,
				"description": cantonInput.Description,
				"signature_request": map[string]any{
					"payload": hex.EncodeToString(sighashes[0].Payload),
				},
				"tx": hex.EncodeToString(serializedInput),
			}))
			return nil
		},
	}

	cmd.Flags().StringVar(&privateKeyRef, "key", "env:"+signer.EnvPrivateKey, "Secret reference for the private key")
	return cmd
}
