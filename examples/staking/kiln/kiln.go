package main

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	xc "github.com/cordialsys/crosschain"
	"github.com/cordialsys/crosschain/chain/evm/abi/stake_batch_deposit"
	evmbuilder "github.com/cordialsys/crosschain/chain/evm/builder"
	evminput "github.com/cordialsys/crosschain/chain/evm/tx_input"
	xcclient "github.com/cordialsys/crosschain/client"
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

func CmdCompute() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "compute",
		Short: "compute a transaction",
		Args:  cobra.ExactArgs(0),
		RunE: func(cmd *cobra.Command, args []string) error {
			xcFactory := setup.UnwrapXc(cmd.Context())
			chain := setup.UnwrapChain(cmd.Context())
			myabi := stake_batch_deposit.NewAbi()
			_ = myabi
			pubkey := mustHex("0x850f24e0a4b2b5568340891fcaecc2d08a788f03f13d2295419e6860545499a24975f2e4154992ebc401925e93a80b3c")
			cred := mustHex("0x010000000000000000000000273b437645ba723299d07b1bdffcf508be64771f")
			sig := mustHex("0xaa040d894ed815d515737c9da0d6bac20f27fcbb159d11ef14bd6557059a432f92e34f739dd0be8fb37efc6be9cb13880ecbb36dcc599c289cdb89bd69f705bb2616e8c62421c9b019c6307743fe437eccaa09dd377dcc33e457b0b3c4c7aa4b")
			expected := "6dff1e04a432e06035343935ad7dacecd938a66e7a6800f548162c19fc72622c"
			balance := xc.NewAmountBlockchainFromStr("32000000000000000000")
			depositDataHash, err := stake_batch_deposit.CalculateDepositDataRoot(balance, pubkey, cred, sig)
			if err != nil {
				return err
			}

			data, err := stake_batch_deposit.Serialize(chain, [][]byte{pubkey}, [][]byte{cred}, [][]byte{sig})
			if err != nil {
				return err
			}

			fmt.Println("expected = ", expected)
			fmt.Println("recieved = ", hex.EncodeToString(depositDataHash))

			fmt.Println("data =", hex.EncodeToString(data))

			privateKeyInput := os.Getenv("PRIVATE_KEY")
			if privateKeyInput == "" {
				return fmt.Errorf("must set env PRIVATE_KEY")
			}
			addressBuilder, err := xcFactory.NewAddressBuilder(chain)
			if err != nil {
				return fmt.Errorf("could not create address builder: %v", err)
			}
			signer, _ := xcFactory.NewSigner(chain)
			fromPrivateKey := xcFactory.MustPrivateKey(chain, privateKeyInput)
			publicKey, err := signer.PublicKey(fromPrivateKey)
			if err != nil {
				return fmt.Errorf("could not create public key: %v", err)
			}

			from, err := addressBuilder.GetAddressFromPublicKey(publicKey)
			if err != nil {
				return fmt.Errorf("could not derive address: %v", err)
			}
			fmt.Println(from)
			to := "0x0866af1D55bb1e9c2f63b1977926276F8d51b806"

			rpcCli, err := xcFactory.NewClient(chain)
			if err != nil {
				return err
			}
			stakingInput := evminput.KilnStakingInput{
				StakingInputEnvelope: evminput.NewKilnStakingInput().StakingInputEnvelope,
				PublicKeys:           [][]byte{pubkey},
				Credentials:          [][]byte{cred},
				Signatures:           [][]byte{sig},
				Amount:               balance,
			}
			rpcCli.(xcclient.SetStakingInput).SetStakingInput(&stakingInput)

			input, err := rpcCli.FetchTxInput(context.Background(), from, xc.Address(to))
			if err != nil {
				return err
			}

			txBuilder := evmbuilder.NewEvmTxBuilder()
			tx, err := txBuilder.BuildTxWithPayload(chain, xc.Address(to), balance, data, input)
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
	return cmd
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

			cli, err := NewClient(chain, xc.StakingVariantEvmKiln, staking.KilnApi, staking.ApiKey)
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

			cli, err := NewClient(chain, xc.StakingVariantEvmKiln, staking.KilnApi, staking.ApiKey)
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

			stakingInput, err := cli.FetchStakeInput(cmd.Context(), from, validator, amount)
			if err != nil {
				return err
			}
			to := "0x0866af1D55bb1e9c2f63b1977926276F8d51b806"
			stakingInput.SetOwner(from)
			stakingInput.SetContract(xc.ContractAddress(to))
			jsonprint(stakingInput)

			rpcCli, err := xcFactory.NewClient(chain)
			if err != nil {
				return err
			}
			rpcCli.(xcclient.SetStakingInput).SetStakingInput(stakingInput)

			txInput, err := rpcCli.FetchTxInput(context.Background(), from, xc.Address(to))
			if err != nil {
				return err
			}
			jsonprint(txInput)

			evmStakingInput := stakingInput.(*evminput.KilnStakingInput)
			data, err := stake_batch_deposit.Serialize(chain, evmStakingInput.PublicKeys, evmStakingInput.Credentials, evmStakingInput.Signatures)
			if err != nil {
				return err
			}

			txBuilder := evmbuilder.NewEvmTxBuilder()
			tx, err := txBuilder.BuildTxWithPayload(chain, xc.Address(to), amount, data, txInput)
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
