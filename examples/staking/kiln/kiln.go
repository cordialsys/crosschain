package kiln

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	xc "github.com/cordialsys/crosschain"
	"github.com/cordialsys/crosschain/builder"
	"github.com/cordialsys/crosschain/client/services"
	"github.com/cordialsys/crosschain/client/services/kiln"
	"github.com/cordialsys/crosschain/cmd/xc/setup"
	"github.com/pelletier/go-toml/v2"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

func jsonprint(a any) {
	bz, _ := json.MarshalIndent(a, "", "  ")
	fmt.Println(string(bz))
}

func mustHex(s string) []byte {
	s = strings.TrimPrefix(s, "0x")
	bz, err := hex.DecodeString(s)
	if err != nil {
		panic(err)
	}
	return bz
}

var _ = mustHex

func getProvider(chain *xc.ChainConfig, providerId string) (xc.StakingProvider, error) {
	for _, provider := range chain.Staking.Providers {
		if strings.EqualFold(providerId, string(provider)) {
			return provider, nil
		}
	}
	return "", fmt.Errorf("unsupported provider %s on chain %s; options: %v", providerId, chain.Chain, chain.Staking.Providers)

}

func CmdStake() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "stake",
		Short: "Stake an asset.",
		Args:  cobra.ExactArgs(0),
		RunE: func(cmd *cobra.Command, args []string) error {
			xcFactory := setup.UnwrapXc(cmd.Context())
			chain := setup.UnwrapChain(cmd.Context())
			moreArgs := setup.UnwrapStakingArgs(cmd.Context())
			stakingCfg := setup.UnwrapStakingConfig(cmd.Context())
			provider, err := getProvider(chain, string(moreArgs.Provider))
			if err != nil {
				return err
			}

			offline, err := cmd.Flags().GetBool("offline")
			if err != nil {
				return err
			}

			validator, err := cmd.Flags().GetString("validator")
			if err != nil {
				return err
			}
			amountHuman := moreArgs.Amount
			if amountHuman.String() == "0" {
				return fmt.Errorf("must pass --amount to stake")
			}
			amount := amountHuman.ToBlockchain(chain.Decimals)

			txBuilder, err := xcFactory.NewTxBuilder(chain)
			if err != nil {
				return err
			}
			stakingBuilder, ok := txBuilder.(builder.Staking)
			if !ok {
				return fmt.Errorf("crosschain currently does not support crafting staking transactions for %s", chain.Chain)
			}
			rpcCli, err := xcFactory.NewClient(chain)
			if err != nil {
				return err
			}
			cli, err := xcFactory.NewStakingClient(stakingCfg, chain, provider)
			if err != nil {
				return err
			}

			privateKeyInput := os.Getenv("PRIVATE_KEY")
			if privateKeyInput == "" {
				return fmt.Errorf("must set env PRIVATE_KEY")
			}
			signer, err := xcFactory.NewSigner(chain, privateKeyInput)
			if err != nil {
				return fmt.Errorf("could not import private key: %v", err)
			}
			publicKey, err := signer.PublicKey()
			if err != nil {
				return fmt.Errorf("could not create public key: %v", err)
			}

			addressBuilder, err := xcFactory.NewAddressBuilder(chain)
			if err != nil {
				return fmt.Errorf("could not create address builder: %v", err)
			}

			from, err := addressBuilder.GetAddressFromPublicKey(publicKey)
			if err != nil {
				return fmt.Errorf("could not derive address: %v", err)
			}
			logrus.WithField("from", from).Debug("sending from")
			options := []builder.BuilderOption{}
			if validator != "" {
				options = append(options, builder.OptionValidator(validator))
			}
			if moreArgs.AccountId != "" {
				options = append(options, builder.OptionStakeAccount(moreArgs.AccountId))
			}
			stakingArgs, err := builder.NewStakeArgs(chain.Chain, from, amount, options...)
			if err != nil {
				return err
			}

			stakingInput, err := cli.FetchStakingInput(cmd.Context(), stakingArgs)
			if err != nil {
				return err
			}

			tx, err := stakingBuilder.Stake(stakingArgs, stakingInput)
			if err != nil {
				return err
			}
			logrus.WithField("tx", tx).Debug("built tx")

			sighashes, err := tx.Sighashes()
			if err != nil {
				return fmt.Errorf("could not create payloads to sign: %v", err)
			}

			// sign
			signatures := []xc.TxSignature{}
			for _, sighash := range sighashes {
				// sign the tx sighash(es)
				signature, err := signer.Sign(sighash)
				if err != nil {
					panic(err)
				}
				signatures = append(signatures, signature)
			}

			err = tx.AddSignatures(signatures...)
			if err != nil {
				return fmt.Errorf("could not add signature(s): %v", err)
			}

			bz, err := tx.Serialize()
			if err != nil {
				return err
			}
			fmt.Println(hex.EncodeToString(bz))
			if offline {
				return nil
			}

			err = rpcCli.SubmitTx(context.Background(), tx)
			if err != nil {
				return fmt.Errorf("could not broadcast: %v", err)
			}
			fmt.Println("submitted tx", tx.Hash())

			return nil
		},
	}
	cmd.Flags().String("validator", "", "the validator address to delegated to, if relevant")
	cmd.Flags().Bool("offline", false, "do not broadcast the signed transaction")
	return cmd
}

func CmdUnstake() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "unstake",
		Short: "Unstake assets from your address.",
		Args:  cobra.ExactArgs(0),
		RunE: func(cmd *cobra.Command, args []string) error {
			xcFactory := setup.UnwrapXc(cmd.Context())
			chain := setup.UnwrapChain(cmd.Context())
			moreArgs := setup.UnwrapStakingArgs(cmd.Context())
			stakingCfg := setup.UnwrapStakingConfig(cmd.Context())
			provider, err := getProvider(chain, string(moreArgs.Provider))
			if err != nil {
				return err
			}

			offline, err := cmd.Flags().GetBool("offline")
			if err != nil {
				return err
			}

			validator, err := cmd.Flags().GetString("validator")
			if err != nil {
				return err
			}
			amountHuman := moreArgs.Amount
			if amountHuman.String() == "0" {
				return fmt.Errorf("must pass --amount to stake")
			}
			amount := amountHuman.ToBlockchain(chain.Decimals)

			txBuilder, err := xcFactory.NewTxBuilder(chain)
			if err != nil {
				return err
			}
			stakingBuilder, ok := txBuilder.(builder.Staking)
			if !ok {
				return fmt.Errorf("crosschain currently does not support crafting staking transactions for %s", chain.Chain)
			}
			rpcCli, err := xcFactory.NewClient(chain)
			if err != nil {
				return err
			}
			cli, err := xcFactory.NewStakingClient(stakingCfg, chain, provider)
			if err != nil {
				return err
			}

			privateKeyInput := os.Getenv("PRIVATE_KEY")
			if privateKeyInput == "" {
				return fmt.Errorf("must set env PRIVATE_KEY")
			}
			signer, err := xcFactory.NewSigner(chain, privateKeyInput)
			if err != nil {
				return fmt.Errorf("could not import private key: %v", err)
			}
			publicKey, err := signer.PublicKey()
			if err != nil {
				return fmt.Errorf("could not create public key: %v", err)
			}

			addressBuilder, err := xcFactory.NewAddressBuilder(chain)
			if err != nil {
				return fmt.Errorf("could not create address builder: %v", err)
			}

			from, err := addressBuilder.GetAddressFromPublicKey(publicKey)
			if err != nil {
				return fmt.Errorf("could not derive address: %v", err)
			}
			logrus.WithField("from", from).Debug("sending from")
			options := []builder.BuilderOption{}
			if validator != "" {
				options = append(options, builder.OptionValidator(validator))
			}
			if moreArgs.AccountId != "" {
				options = append(options, builder.OptionStakeAccount(moreArgs.AccountId))
			}
			stakingArgs, err := builder.NewStakeArgs(chain.Chain, from, amount, options...)
			if err != nil {
				return err
			}

			stakingInput, err := cli.FetchUnstakingInput(cmd.Context(), stakingArgs)
			if err != nil {
				return err
			}

			tx, err := stakingBuilder.Unstake(stakingArgs, stakingInput)
			if err != nil {
				return err
			}
			logrus.WithField("tx", tx).Debug("built tx")

			sighashes, err := tx.Sighashes()
			if err != nil {
				return fmt.Errorf("could not create payloads to sign: %v", err)
			}

			// sign
			signatures := []xc.TxSignature{}
			for _, sighash := range sighashes {
				// sign the tx sighash(es)
				signature, err := signer.Sign(sighash)
				if err != nil {
					panic(err)
				}
				signatures = append(signatures, signature)
			}

			err = tx.AddSignatures(signatures...)
			if err != nil {
				return fmt.Errorf("could not add signature(s): %v", err)
			}

			bz, err := tx.Serialize()
			if err != nil {
				return err
			}
			fmt.Println(hex.EncodeToString(bz))
			if offline {
				return nil
			}

			err = rpcCli.SubmitTx(context.Background(), tx)
			if err != nil {
				return fmt.Errorf("could not broadcast: %v", err)
			}
			fmt.Println("submitted tx", tx.Hash())

			return nil
		},
	}
	cmd.Flags().String("validator", "", "the validator address to delegated to, if relevant")
	cmd.Flags().Bool("offline", false, "do not broadcast the signed transaction")
	return cmd
}

func CmdKilnTest() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "test",
		Short: "Using kiln provider.",
		Args:  cobra.ExactArgs(0),
		RunE: func(cmd *cobra.Command, args []string) error {
			xcFactory := setup.UnwrapXc(cmd.Context())
			chain := setup.UnwrapChain(cmd.Context())
			moreArgs := setup.UnwrapStakingArgs(cmd.Context())
			stakingCfg := setup.UnwrapStakingConfig(cmd.Context())
			// rpcArgs := setup.UnwrapArgs(cmd.Context())
			_ = xcFactory
			_ = chain
			bal := moreArgs.Amount.ToBlockchain(chain.Decimals)
			_ = bal
			apiKey, err := stakingCfg.Kiln.ApiToken.Load()
			if err != nil {
				return err
			}

			cli, err := kiln.NewClient(string(chain.Chain), stakingCfg.Kiln.BaseUrl, apiKey)
			if err != nil {
				return err
			}
			acc, err := cli.ResolveAccount(moreArgs.AccountId)
			if err != nil {
				return err
			}
			jsonprint(acc)
			stakes, err := cli.GetAllStakesByOwner("0x273b437645Ba723299d07B1BdFFcf508bE64771f")
			if err != nil {
				return err
			}
			jsonprint(stakes)
			// cli.
			// privateKeyInput := os.Getenv("PRIVATE_KEY")
			// if privateKeyInput == "" {
			// 	return fmt.Errorf("must set env PRIVATE_KEY")
			// }
			// signer, err := xcFactory.NewSigner(chain, privateKeyInput)
			// if err != nil {
			// 	return fmt.Errorf("could not import private key: %v", err)
			// }
			// publicKey, err := signer.PublicKey()
			// if err != nil {
			// 	return fmt.Errorf("could not create public key: %v", err)
			// }

			// addressBuilder, err := xcFactory.NewAddressBuilder(chain)
			// if err != nil {
			// 	return fmt.Errorf("could not create address builder: %v", err)
			// }

			// from, err := addressBuilder.GetAddressFromPublicKey(publicKey)
			// if err != nil {
			// 	return fmt.Errorf("could not derive address: %v", err)
			// }
			// fmt.Println(from)

			// keys, err := cli.CreateValidatorKeys(acc.ID, string(from), 1)
			// if err != nil {
			// 	return fmt.Errorf("could not create validator keys: %v", err)
			// }
			// jsonprint(keys)

			// trans, err := cli.GenerateStakeTransaction(acc.ID, string(from), bal)
			// if err != nil {
			// 	return fmt.Errorf("could not generate transaction: %v", err)
			// }
			// jsonprint(trans)

			return nil
		},
	}
	return cmd
}

func CmdConfig() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config",
		Short: "Show staking configuration",
		Args:  cobra.ExactArgs(0),
		RunE: func(cmd *cobra.Command, args []string) error {
			xcFactory := setup.UnwrapXc(cmd.Context())
			// chain := setup.UnwrapChain(cmd.Context())
			// staking := setup.UnwrapStakingArgs(cmd.Context())

			stakingConfig, err := services.LoadConfig(xcFactory.GetNetworkSelector())
			if err != nil {
				return err
			}
			bz, err := toml.Marshal(stakingConfig)
			if err != nil {
				return err
			}
			fmt.Println(string(bz))

			return nil
		},
	}
	return cmd
}
