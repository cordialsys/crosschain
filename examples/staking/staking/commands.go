package staking

import (
	"context"
	"encoding/hex"
	"fmt"
	"os"

	"github.com/cordialsys/crosschain"
	xc "github.com/cordialsys/crosschain"
	"github.com/cordialsys/crosschain/builder"
	"github.com/cordialsys/crosschain/cmd/xc/setup"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

func CmdStakedBalances() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "balance <address>",
		Short: "Lookup staked balances.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			xcFactory := setup.UnwrapXc(cmd.Context())
			chain := setup.UnwrapChain(cmd.Context())
			stakingCfg := setup.UnwrapStakingConfig(cmd.Context())
			from := args[0]

			client, err := xcFactory.NewStakingClient(stakingCfg, chain, xc.Native)
			if err != nil {
				return err
			}

			balances, err := client.FetchStakeBalance(context.Background(), xc.Address(from), "", "")
			if err != nil {
				return err
			}

			jsonprint(balances)

			return nil
		},
	}
	// cmd.Flags().String("validator", "", "the validator address to delegated to, if relevant")
	return cmd
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
			validator, err := cmd.Flags().GetString("validator")
			if err != nil {
				return err
			}
			offline, err := cmd.Flags().GetBool("offline")
			if err != nil {
				return err
			}

			amountHuman := moreArgs.Amount
			if amountHuman.String() == "0" {
				return fmt.Errorf("must pass --amount to stake")
			}
			amount := amountHuman.ToBlockchain(chain.Decimals)

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

			txBuilder, err := xcFactory.NewTxBuilder(chain)
			if err != nil {
				return err
			}
			stakingBuilder, ok := txBuilder.(builder.Staking)
			if !ok {
				return fmt.Errorf("crosschain currently does not support crafting staking transactions for %s", chain.Chain)
			}

			client, err := xcFactory.NewStakingClient(stakingCfg, chain, crosschain.Native)
			if err != nil {
				return err
			}

			options := []builder.StakeOption{}
			if validator != "" {
				options = append(options, builder.StakeOptionValidator(validator))
			}
			if moreArgs.AccountId != "" {
				options = append(options, builder.StakeOptionAccountId(moreArgs.AccountId))
			}
			stakingArgs, err := builder.NewStakeArgs(from, amount, options...)
			if err != nil {
				return err
			}

			stakingInput, err := client.FetchStakingInput(cmd.Context(), stakingArgs)
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
				// end before submitting
				return nil
			}

			rpcCli, err := xcFactory.NewClient(chain)
			if err != nil {
				return err
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

func CmdFetchStakeInput() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "stake-input <address> <amount>",
		Short: "Looking inputs for a new staking transaction.",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			xcFactory := setup.UnwrapXc(cmd.Context())
			chain := setup.UnwrapChain(cmd.Context())
			moreArgs := setup.UnwrapStakingArgs(cmd.Context())
			stakingCfg := setup.UnwrapStakingConfig(cmd.Context())
			validator, err := cmd.Flags().GetString("validator")
			if err != nil {
				return err
			}

			from := args[0]
			amount := xc.NewAmountBlockchainFromStr(args[1])

			options := []builder.StakeOption{}
			if validator != "" {
				options = append(options, builder.StakeOptionValidator(validator))
			}
			if moreArgs.AccountId != "" {
				options = append(options, builder.StakeOptionAccountId(moreArgs.AccountId))
			}

			client, err := xcFactory.NewStakingClient(stakingCfg, chain, xc.Native)
			if err != nil {
				return err
			}

			stakeArgs, err := builder.NewStakeArgs(xc.Address(from), amount, options...)
			if err != nil {
				return err
			}

			input, err := client.FetchStakingInput(context.Background(), stakeArgs)
			if err != nil {
				return err
			}

			jsonprint(input)

			return nil
		},
	}
	cmd.Flags().String("validator", "", "the validator address to delegated to, if relevant")
	return cmd
}

func CmdFetchUnStakeInput() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "unstake-input <address> <amount>",
		Short: "Looking inputs for a new unstake transaction.",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			xcFactory := setup.UnwrapXc(cmd.Context())
			chain := setup.UnwrapChain(cmd.Context())
			moreArgs := setup.UnwrapStakingArgs(cmd.Context())
			stakingCfg := setup.UnwrapStakingConfig(cmd.Context())
			validator, err := cmd.Flags().GetString("validator")
			if err != nil {
				return err
			}

			from := args[0]
			amount := xc.NewAmountBlockchainFromStr(args[1])

			options := []builder.StakeOption{}
			if validator != "" {
				options = append(options, builder.StakeOptionValidator(validator))
			}
			if moreArgs.AccountId != "" {
				options = append(options, builder.StakeOptionAccountId(moreArgs.AccountId))
			}

			client, err := xcFactory.NewStakingClient(stakingCfg, chain, xc.Native)
			if err != nil {
				return err
			}

			stakeArgs, err := builder.NewStakeArgs(xc.Address(from), amount, options...)
			if err != nil {
				return err
			}

			input, err := client.FetchUnstakingInput(context.Background(), stakeArgs)
			if err != nil {
				return err
			}

			jsonprint(input)

			return nil
		},
	}
	cmd.Flags().String("validator", "", "the validator address to delegated to, if relevant")
	return cmd
}
