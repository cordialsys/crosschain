package main

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	xc "github.com/cordialsys/crosschain"
	"github.com/cordialsys/crosschain/builder"
	"github.com/cordialsys/crosschain/chain/evm/client/staking/kiln"
	"github.com/cordialsys/crosschain/cmd/xc/setup"
	"github.com/cordialsys/crosschain/examples/staking/kiln/api"
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

func CmdGetStake() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "get-stake",
		Short: "Lookup balance states of a stake account.",
		Args:  cobra.ExactArgs(0),
		RunE: func(cmd *cobra.Command, args []string) error {
			xcFactory := setup.UnwrapXc(cmd.Context())
			chain := setup.UnwrapChain(cmd.Context())
			staking := setup.UnwrapStakingArgs(cmd.Context())

			owner, err := cmd.Flags().GetString("owner")
			if err != nil {
				return err
			}
			validator, err := cmd.Flags().GetString("validator")
			if err != nil {
				return err
			}
			stake, err := cmd.Flags().GetString("stake")
			if err != nil {
				return err
			}
			if owner == "" && validator == "" && stake == "" {
				return fmt.Errorf("must provide at least one of --owner, --validator, or --stake")
			}
			// rpcArgs := setup.UnwrapArgs(cmd.Context())
			_ = xcFactory
			_ = chain

			cli, err := kiln.NewClient(chain, xc.StakingVariantEvmKiln, staking.KilnApi, staking.ApiKey)
			if err != nil {
				return err
			}

			bal, err := cli.FetchStakeBalance(cmd.Context(), xc.Address(owner), validator, xc.Address(stake))
			if err != nil {
				return err
			}
			jsonprint(bal)

			return nil
		},
	}
	cmd.Flags().String("owner", "", "address owning the stake account")
	cmd.Flags().String("validator", "", "the validator address delegated to")
	cmd.Flags().String("stake", "", "the address of the stake account")
	return cmd
}

func CmdStake() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "stake <amount>",
		Short: "Stake an asset.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			xcFactory := setup.UnwrapXc(cmd.Context())
			chain := setup.UnwrapChain(cmd.Context())
			staking := setup.UnwrapStakingArgs(cmd.Context())

			validator, err := cmd.Flags().GetString("validator")
			if err != nil {
				return err
			}

			amountHuman, err := xc.NewAmountHumanReadableFromStr(args[0])
			if err != nil {
				return fmt.Errorf("invalid amount: %v", err)
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

			cli, err := kiln.NewClient(chain, xc.StakingVariantEvmKiln, staking.KilnApi, staking.ApiKey)
			if err != nil {
				return err
			}

			privateKeyInput := os.Getenv("PRIVATE_KEY")
			if privateKeyInput == "" {
				return fmt.Errorf("must set env PRIVATE_KEY")
			}
			fromPrivateKey := xcFactory.MustPrivateKey(chain, privateKeyInput)
			signer, _ := xcFactory.NewSigner(chain)
			publicKey, err := signer.PublicKey(fromPrivateKey)
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
			fmt.Println(from)
			stakingArgs, err := builder.NewStakeArgs(from, amount, builder.StakeOptionValidator(validator))
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
			fmt.Println("built tx", tx)

			sighashes, err := tx.Sighashes()
			if err != nil {
				return fmt.Errorf("could not create payloads to sign: %v", err)
			}

			// sign
			signatures := []xc.TxSignature{}
			for _, sighash := range sighashes {
				// sign the tx sighash(es)
				signature, err := signer.Sign(fromPrivateKey, sighash)
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

			err = rpcCli.SubmitTx(context.Background(), tx)
			if err != nil {
				return fmt.Errorf("could not broadcast: %v", err)
			}
			fmt.Println("submitted tx", tx.Hash())

			return nil
		},
	}
	cmd.Flags().String("validator", "", "the validator address to delegated to, if relevant")
	return cmd
}

func CmdKiln() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "kiln",
		Short: "Using kiln provider.",
		Args:  cobra.ExactArgs(0),
		RunE: func(cmd *cobra.Command, args []string) error {
			xcFactory := setup.UnwrapXc(cmd.Context())
			chain := setup.UnwrapChain(cmd.Context())
			staking := setup.UnwrapStakingArgs(cmd.Context())
			// rpcArgs := setup.UnwrapArgs(cmd.Context())
			_ = xcFactory
			_ = chain
			bal := staking.Amount.ToBlockchain(chain.Decimals)

			cli, err := api.NewClient(string(chain.Chain), staking.KilnApi, staking.ApiKey)
			if err != nil {
				return err
			}
			acc, err := cli.ResolveAccount(staking.AccountId)
			if err != nil {
				return err
			}
			jsonprint(acc)
			privateKeyInput := os.Getenv("PRIVATE_KEY")
			if privateKeyInput == "" {
				return fmt.Errorf("must set env PRIVATE_KEY")
			}
			fromPrivateKey := xcFactory.MustPrivateKey(chain, privateKeyInput)
			signer, _ := xcFactory.NewSigner(chain)
			publicKey, err := signer.PublicKey(fromPrivateKey)
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
			fmt.Println(from)

			keys, err := cli.CreateValidatorKeys(acc.ID, string(from), 1)
			if err != nil {
				return fmt.Errorf("could not create validator keys: %v", err)
			}
			jsonprint(keys)

			trans, err := cli.GenerateStakeTransaction(acc.ID, string(from), bal)
			if err != nil {
				return fmt.Errorf("could not generate transaction: %v", err)
			}
			jsonprint(trans)

			return nil
		},
	}
	return cmd
}
