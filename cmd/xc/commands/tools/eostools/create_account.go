package eostools

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"time"

	xc "github.com/cordialsys/crosschain"
	xcbuilder "github.com/cordialsys/crosschain/builder"
	"github.com/cordialsys/crosschain/chain/eos/address"
	"github.com/cordialsys/crosschain/chain/eos/builder/action"
	eos "github.com/cordialsys/crosschain/chain/eos/eos-go"
	"github.com/cordialsys/crosschain/chain/eos/eos-go/ecc"
	"github.com/cordialsys/crosschain/chain/eos/tx_input"
	xctypes "github.com/cordialsys/crosschain/client/types"
	"github.com/cordialsys/crosschain/cmd/xc/setup"
	"github.com/cordialsys/crosschain/config"
	"github.com/cordialsys/crosschain/factory/signer"
	"github.com/spf13/cobra"
)

func CmdTxCreateAccount() *cobra.Command {
	var dryRun bool
	var fromSecretRef string
	var ramBytes int

	var name string
	var addressAuthorizor string
	var addressOwner string

	cmd := &cobra.Command{
		Use:   "create-account",
		Short: "Create an EOS account for any address.",
		Args:  cobra.ExactArgs(0),
		RunE: func(cmd *cobra.Command, _args []string) error {
			xcFactory := setup.UnwrapXc(cmd.Context())
			chainConfig := setup.UnwrapChain(cmd.Context())
			ctx := cmd.Context()
			_ = xcFactory
			_ = chainConfig
			if name == "" {
				return fmt.Errorf("must set --name")
			}
			if addressAuthorizor == "" {
				return fmt.Errorf("must set --address")
			}
			if addressOwner == "" {
				addressOwner = addressAuthorizor
			}

			ownerKey, err := ecc.NewPublicKey(addressOwner)
			if err != nil {
				return fmt.Errorf("failed to create owner key: %v", err)
			}
			authorizorKey, err := ecc.NewPublicKey(addressAuthorizor)
			if err != nil {
				return fmt.Errorf("failed to create authorizor key: %v", err)
			}

			client, err := xcFactory.NewClient(chainConfig)
			if err != nil {
				return fmt.Errorf("could not load client: %v", err)
			}

			privateKeyInput, err := config.GetSecret(fromSecretRef)
			if err != nil {
				return fmt.Errorf("could not get from-address secret: %v", err)
			}
			if privateKeyInput == "" {
				return fmt.Errorf("must set env %s", signer.EnvPrivateKey)
			}

			mainSigner, err := xcFactory.NewSigner(chainConfig.Base(), privateKeyInput)
			if err != nil {
				return fmt.Errorf("could not import private key: %v", err)
			}
			publicKey, err := mainSigner.PublicKey()
			if err != nil {
				return fmt.Errorf("could not create public key: %v", err)
			}

			builder, err := address.NewAddressBuilder(chainConfig.Base())
			if err != nil {
				return err
			}

			fromAddress, err := builder.GetAddressFromPublicKey(publicKey)
			if err != nil {
				return err
			}
			tfArgs, err := xcbuilder.NewTransferArgs(chainConfig.Base(), fromAddress, xc.Address(fromAddress), xc.NewAmountBlockchainFromUint64(1))
			if err != nil {
				return fmt.Errorf("invalid transfer args: %v", err)
			}

			inputI, err := client.FetchTransferInput(ctx, tfArgs)
			if err != nil {
				return fmt.Errorf("failed to fetch transfer input: %v", err)
			}
			input := inputI.(*tx_input.TxInput)
			input.Timestamp = time.Now().Unix()

			refApi := eos.New(chainConfig.URL, chainConfig.HttpTimeout)
			refApi.Header.Set("Content-Type", "application/json")

			fromAccount := input.FromAccount

			actionCreate, err := action.NewNewAccount(fromAccount, name, eos.Authority{
				Threshold: 1,
				Keys: []eos.KeyWeight{
					{
						PublicKey: ownerKey,
						Weight:    1,
					},
				},
				Accounts: []eos.PermissionLevelWeight{},
			}, eos.Authority{
				Threshold: 1,
				Keys: []eos.KeyWeight{
					{
						PublicKey: authorizorKey,
						Weight:    1,
					},
				},
				Accounts: []eos.PermissionLevelWeight{},
			})
			if err != nil {
				return fmt.Errorf("failed to create new account action: %v", err)
			}

			actionBuyRam, err := action.NewBuyRamBytes(fromAccount, name, uint32(ramBytes))
			if err != nil {
				return fmt.Errorf("failed to create buy ram action: %v", err)
			}

			eosTx := &eos.Transaction{Actions: []*eos.Action{}}
			eosTx.RefBlockNum = uint16(binary.BigEndian.Uint32(input.HeadBlockID[:4]))
			eosTx.RefBlockPrefix = binary.LittleEndian.Uint32(input.HeadBlockID[8:16])
			expiration := time.Unix(input.Timestamp, 0)
			expiration = expiration.Add(tx_input.ExpirationPeriod)
			eosTx.Expiration = eos.JSONTime{Time: expiration}
			eosTx.Actions = []*eos.Action{actionCreate, actionBuyRam}

			signedTx, err := signEosTx(mainSigner, eosTx, input)
			if err != nil {
				return fmt.Errorf("failed to sign transaction: %v", err)
			}

			packedTrx, err := signedTx.Pack(eos.CompressionNone)
			if err != nil {
				return err
			}
			packedTrx.Compression = eos.CompressionNone

			trxID, err := packedTrx.ID()
			if err != nil {
				return err
			}
			fmt.Printf("transaction id: %s\n", trxID)

			packedTrxBytes, err := json.Marshal(packedTrx)
			if err != nil {
				return fmt.Errorf("failed to marshal packed transaction: %v", err)
			}
			binaryTx := xctypes.NewBinaryTx(packedTrxBytes, []byte{})
			serializedTx, err := binaryTx.Serialize()
			if err != nil {
				return fmt.Errorf("failed to serialize transaction: %v", err)
			}

			if dryRun {
				jsonPrint(serializedTx)
				return nil
			}

			out, err := refApi.PushRawTransactionRaw(ctx, serializedTx)
			if err != nil {
				return fmt.Errorf("failed to send transaction: %v", err)
			}
			_ = out
			fmt.Printf("%s accepted\n", trxID)

			return nil
		},
	}
	cmd.Flags().StringVar(&fromSecretRef, "from", "env:"+signer.EnvPrivateKey, "Secret reference for the signer private key (not necessary the address owning the new account)")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Dry run the transaction, printing it, but not submitting it.")
	cmd.Flags().IntVar(&ramBytes, "ram", 3600, "Amount of RAM to buy for the account.")

	cmd.Flags().StringVar(&name, "name", "", "Name of the account to create (12 characters, a-z, 1-5)")
	cmd.Flags().StringVar(&addressAuthorizor, "address", "", "Address to authorize and own the account.")
	cmd.Flags().StringVar(&addressOwner, "address-owner", "", "Optional.  Specify a separate address to own the account.")
	return cmd
}
