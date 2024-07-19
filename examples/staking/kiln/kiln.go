package main

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"

	xc "github.com/cordialsys/crosschain"
	"github.com/cordialsys/crosschain/chain/evm/stake_batch_deposit"
	"github.com/cordialsys/crosschain/cmd/xc/setup"
	"github.com/cordialsys/crosschain/examples/staking/kiln/api"
	"github.com/spf13/cobra"
)

func jsonprint(a any) {
	bz, _ := json.MarshalIndent(a, "", "  ")
	fmt.Println(string(bz))
}

func mustHex(s string) []byte {
	bz, err := hex.DecodeString(s)
	if err != nil {
		panic(err)
	}
	return bz
}

func sum256(datas ...[]byte) []byte {
	h := sha256.New()
	// h := sha3.NewLegacyKeccak256()
	for _, d := range datas {
		_, _ = h.Write(d)
	}
	digest := []byte{}
	return h.Sum(digest)
}
func wei2gwei(amount string) uint64 {
	b := xc.NewAmountBlockchainFromStr(amount)
	// divide by 10**9
	gwei := b.ToHuman(9).ToBlockchain(0).Uint64()
	if gwei == 0 {
		panic("too small amount")
	}
	return gwei
}
func mustLen(data []byte, l int) {
	if len(data) != l {
		panic("wrong length")
	}
}

func CmdCompute() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "compute",
		Short: "compute a transaction",
		Args:  cobra.ExactArgs(0),
		RunE: func(cmd *cobra.Command, args []string) error {
			myabi := stake_batch_deposit.NewAbi()
			_ = myabi
			pubkey := mustHex("8c226ab28b514ec37ff069ea7c7b4dab0b359ef7992204d8dfadca230591be181eb3f3450058b8df79aef6bbae1ec5aa")
			cred := mustHex("010000000000000000000000273b437645ba723299d07b1bdffcf508be64771f")
			sig := mustHex("a8bd69560369e1aaac1ed51406eaf79747dcdd8b75fd2d4c17cb054cb07da42cd87ab9c2de2d4909c6bc9c287573df4709b86281565f7bdff2b630b1b7dbadde91704f478c93097e6ba2393c7a4b068095add898527a5c75884c8e440e9cb8d5")
			expected := "615048dff044f1969659b5a197a1979a3b0ed3487a8d30996a9f2bdcfc178f0f"
			amount := wei2gwei("32000000000000000000")

			mustLen(pubkey, 48)
			mustLen(cred, 32)
			mustLen(sig, 96)

			amountBz := make([]byte, 8)
			binary.LittleEndian.PutUint64(amountBz, amount)
			fmt.Println("amount: ", binary.LittleEndian.Uint64(amountBz))

			pubkeyRoot := sum256(pubkey, make([]byte, 16))
			signaureRoot := sum256(
				sum256(sig[:64]),
				sum256(sig[64:], make([]byte, 32)),
			)
			node := sum256(
				sum256(pubkeyRoot, cred),
				sum256(amountBz, make([]byte, 24), signaureRoot),
			)

			fmt.Println("expected = ", expected)
			fmt.Println("recieved = ", hex.EncodeToString(node))

			return nil
		},
	}
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
