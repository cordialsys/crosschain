// Copyright 2024 github.com/cordialsys
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package solana

import (
	"context"
	"encoding/json"
	"fmt"

	xc "github.com/cordialsys/crosschain"
	"github.com/cordialsys/crosschain/builder"
	xctypes "github.com/cordialsys/crosschain/chain/crosschain/types"
	"github.com/cordialsys/crosschain/chain/solana/tx_input"
	"github.com/cordialsys/crosschain/cmd/xc/setup"
	"github.com/cordialsys/crosschain/factory/signer"
	"github.com/gagliardetto/solana-go"
	compute_budget "github.com/gagliardetto/solana-go/programs/compute-budget"
	"github.com/gagliardetto/solana-go/programs/stake"
	"github.com/gagliardetto/solana-go/programs/system"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

func jsonprint(a any) {
	bz, _ := json.MarshalIndent(a, "", "  ")
	fmt.Println(string(bz))
}

func buildSolanaTx(instructions []solana.Instruction, accountFrom solana.PublicKey, recentHash solana.Hash) (*solana.Transaction, error) {
	tx, err := solana.NewTransaction(
		instructions,
		recentHash,
		solana.TransactionPayer(accountFrom),
	)
	if err != nil {
		return nil, err
	}
	return tx, nil
}

func CmdSolana() *cobra.Command {
	cmd := &cobra.Command{
		Use:          "solana",
		Short:        "Using solana provider",
		Args:         cobra.ExactArgs(0),
		SilenceUsage: true,
	}

	cmd.AddCommand(CmdStake())
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

			amountHuman := moreArgs.Amount
			if amountHuman != nil {
				return fmt.Errorf("must pass --amount to stake")
			}
			amount := amountHuman.ToBlockchain(chain.Decimals)

			privateKeyInput := signer.ReadPrivateKeyEnv()
			if privateKeyInput == "" {
				return fmt.Errorf("must set env %s", signer.EnvPrivateKey)
			}
			signer, err := xcFactory.NewSigner(chain.Base(), privateKeyInput)
			if err != nil {
				return fmt.Errorf("could not import private key: %v", err)
			}
			publicKey, err := signer.PublicKey()
			if err != nil {
				return fmt.Errorf("could not create public key: %v", err)
			}

			addressBuilder, err := xcFactory.NewAddressBuilder(chain.Base())
			if err != nil {
				return fmt.Errorf("could not create address builder: %v", err)
			}

			from, err := addressBuilder.GetAddressFromPublicKey(publicKey)
			if err != nil {
				return fmt.Errorf("could not derive address: %v", err)
			}
			logrus.WithField("from", from).Debug("sending from")

			_ = stakingCfg
			_ = amount
			stakeKey, err := solana.NewRandomPrivateKey()
			if err != nil {
				return err
			}

			_ = stakeKey
			fromPub, err := solana.PublicKeyFromBase58(string(from))
			if err != nil {
				return err
			}
			validatorVoteAcc, err := solana.PublicKeyFromBase58("he1iusunGwqrNtafDtLdhsUQDFvo13z9sUa36PauBtk")
			if err != nil {
				return err
			}

			instructions := []solana.Instruction{}
			instructions = append(instructions,
				compute_budget.NewSetComputeUnitPriceInstruction(100000).Build(),
			)
			instructions = append(instructions,
				system.NewCreateAccountInstruction(amount.Uint64(), 200, solana.StakeProgramID, fromPub, stakeKey.PublicKey()).Build(),
			)
			instructions = append(instructions,
				stake.NewInitializeInstruction(fromPub, fromPub, stakeKey.PublicKey()).Build(),
			)
			instructions = append(instructions,
				stake.NewDelegateStakeInstruction(validatorVoteAcc, fromPub, stakeKey.PublicKey()).Build(),
			)
			client, err := xcFactory.NewClient(chain)
			if err != nil {
				return err
			}
			tfArgs, err := builder.NewTransferArgs(chain.Base(), from, "", amount)
			if err != nil {
				return err
			}
			input, err := client.FetchTransferInput(context.Background(), tfArgs)
			if err != nil {
				return err
			}
			recentHash := input.(*tx_input.TxInput).RecentBlockHash

			tx, err := buildSolanaTx(instructions, fromPub, recentHash)
			if err != nil {
				return err
			}
			signBody, err := tx.Message.MarshalBinary()
			if err != nil {
				return err
			}
			sig1, err := signer.Sign(&xc.SignatureRequest{
				Payload: signBody,
				Signer:  from,
			})
			if err != nil {
				return err
			}
			sig2, err := stakeKey.Sign(signBody)
			if err != nil {
				return err
			}
			tx.Signatures = append(tx.Signatures, solana.Signature(sig1.Signature), sig2)
			tzBz, err := tx.MarshalBinary()
			if err != nil {
				return err
			}
			fmt.Println("submitting hash ", solana.Signature(sig1.Signature).String(), "...")

			err = client.SubmitTx(context.Background(), xctypes.NewBinaryTx(tzBz, nil))
			if err != nil {
				return err
			}

			return nil
		},
	}
	cmd.Flags().String("validator", "", "the validator address to delegated to, if relevant")
	cmd.Flags().Bool("offline", false, "do not broadcast the signed transaction")
	return cmd
}
