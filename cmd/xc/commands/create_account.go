package commands

import (
	"bytes"
	"fmt"
	"time"

	xc "github.com/cordialsys/crosschain"
	xcbuilder "github.com/cordialsys/crosschain/builder"
	xclient "github.com/cordialsys/crosschain/client"
	xcerrors "github.com/cordialsys/crosschain/client/errors"
	"github.com/cordialsys/crosschain/cmd/xc/setup"
	"github.com/cordialsys/crosschain/config"
	"github.com/cordialsys/crosschain/factory/drivers"
	"github.com/cordialsys/crosschain/factory/signer"
	fsigner "github.com/cordialsys/crosschain/factory/signer"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

func CmdCreateAccount() *cobra.Command {
	var privateKeyRef string
	var timeout time.Duration

	cmd := &cobra.Command{
		Use:   "create-account",
		Short: "Create or register an account end-to-end when the chain requires it.",
		Args:  cobra.ExactArgs(0),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
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
			signerCollection := fsigner.NewCollection()

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
			signerCollection.AddMainSigner(xcSigner, address)

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
			builderArgs, err := xcbuilder.NewCreateAccountArgs(chainConfig.Chain, address, publicKey)
			if err != nil {
				return fmt.Errorf("could not build create-account args: %v", err)
			}

			start := time.Now()
			for time.Since(start) < timeout {
				state, err := accountClient.GetAccountState(ctx, createArgs)
				if err != nil {
					return fmt.Errorf("could not fetch account state: %v", err)
				}

				switch state {
				case xclient.AccountInactive:
					logrus.Info("account creation step required")

					input, err := accountClient.FetchCreateAccountInput(ctx, createArgs)
					if err != nil {
						if xcerrors.Is(err, xcerrors.AddressAlreadyActive) {
							logrus.Info("account is already active, re-polling state...")
							time.Sleep(2 * time.Second)
							return nil
						}
						return fmt.Errorf("could not fetch create-account input: %v", err)
					}

					// verify we can marshal/unmarshal the input
					inputBz, err := drivers.MarshalVariantInput(input)
					if err != nil {
						return fmt.Errorf("could not marshal create-account input: %v", err)
					}
					_, err = drivers.UnmarshalVariantInput(inputBz)
					if err != nil {
						return fmt.Errorf("could not unmarshal create-account input: %v", err)
					}

					tx, err := prepareCreateAccountForSubmit(accountBuilder, builderArgs, input, signerCollection)
					if err != nil {
						return fmt.Errorf("could not prepare create-account tx: %v", err)
					}

					// Run safety check to make sure there's no non-deterministic behavior
					for i := range 10 {
						if i == 5 {
							time.Sleep(1 * time.Second)
						}
						tx2, err := prepareCreateAccountForSubmit(accountBuilder, builderArgs, input, signerCollection)
						if err != nil {
							return fmt.Errorf("could not prepare additional create-account tx: %v", err)
						}
						if tx2.Hash() != tx.Hash() {
							// This would be a bad bug -- the transaction builder should always be deterministic.
							return fmt.Errorf("create-account tx hash mismatch, non-deterministic transaction builder")
						}
						sighashes1, err := tx.Sighashes()
						if err != nil {
							return fmt.Errorf("could not create payloads to sign: %v", err)
						}
						sighashes2, err := tx2.Sighashes()
						if err != nil {
							return fmt.Errorf("could not create payloads to sign: %v", err)
						}
						if len(sighashes1) != len(sighashes2) {
							return fmt.Errorf("create-account tx sighash count mismatch, non-deterministic transaction builder")
						}
						for i := range sighashes1 {
							if !bytes.Equal(sighashes1[i].Payload, sighashes2[i].Payload) {
								return fmt.Errorf("create-account tx sighash payload mismatch, non-deterministic transaction builder")
							}
						}
					}

					if err := SubmitTransaction(chainConfig.Chain, rpcClient, tx, timeoutRemaining(start, timeout)); err != nil {
						return fmt.Errorf("could not submit create-account tx: %v", err)
					}
					continue
				case xclient.AccountRegistering:
					logrus.Info("account creation is pending, waiting 10s")
					time.Sleep(10 * time.Second)
					continue
				case xclient.AccountActive:
					fmt.Println(asJson(map[string]any{
						"address": string(address),
						"chain":   string(chainConfig.Chain),
						"status":  "registered",
						"state":   state,
					}))
					return nil
				default:
					return fmt.Errorf("unsupported account state %q", state)
				}
			}

			return fmt.Errorf("timed out waiting for account creation after %s", timeout)
		},
	}

	cmd.Flags().StringVar(&privateKeyRef, "key", "env:"+signer.EnvPrivateKey, "Secret reference for the private key")
	cmd.Flags().DurationVar(&timeout, "timeout", 2*time.Minute, "Amount of time to wait for account creation to complete.")
	return cmd
}

func prepareCreateAccountForSubmit(accountBuilder xcbuilder.AccountCreation, builderArgs xcbuilder.CreateAccountArgs, input xc.CreateAccountTxInput, signerCollection *fsigner.Collection) (xc.Tx, error) {
	tx, err := accountBuilder.CreateAccount(builderArgs, input)
	if err != nil {
		return nil, fmt.Errorf("could not build create-account tx: %v", err)
	}

	signatures := []*xc.SignatureResponse{}
	sighashes, err := tx.Sighashes()
	if err != nil {
		return nil, fmt.Errorf("could not create payloads to sign: %v", err)
	}
	if len(sighashes) == 0 {
		return nil, fmt.Errorf("create-account tx produced no sighashes")
	}
	for _, sighash := range sighashes {
		signature, err := signerCollection.Sign(sighash.Signer, sighash.Payload)
		if err != nil {
			return nil, err
		}
		signatures = append(signatures, signature)
	}

	tx, err = accountBuilder.CreateAccount(builderArgs, input)
	if err != nil {
		return nil, fmt.Errorf("could not rebuild create-account tx for serialization: %v", err)
	}
	if err := tx.SetSignatures(signatures...); err != nil {
		return nil, fmt.Errorf("could not add signature(s): %v", err)
	}
	return tx, nil
}

func timeoutRemaining(start time.Time, total time.Duration) time.Duration {
	remaining := total - time.Since(start)
	if remaining <= 0 {
		return time.Second
	}
	return remaining
}
