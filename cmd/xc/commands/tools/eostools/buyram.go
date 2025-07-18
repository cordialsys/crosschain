package eostools

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"time"

	xc "github.com/cordialsys/crosschain"
	xcbuilder "github.com/cordialsys/crosschain/builder"
	xctypes "github.com/cordialsys/crosschain/chain/crosschain/types"
	"github.com/cordialsys/crosschain/chain/eos/address"
	"github.com/cordialsys/crosschain/chain/eos/builder/action"
	eos "github.com/cordialsys/crosschain/chain/eos/eos-go"
	"github.com/cordialsys/crosschain/chain/eos/tx_input"
	"github.com/cordialsys/crosschain/cmd/xc/setup"
	"github.com/cordialsys/crosschain/config"
	"github.com/cordialsys/crosschain/factory/signer"
	"github.com/spf13/cobra"
)

func CmdTxBuyRam() *cobra.Command {
	var dryRun bool
	var fromSecretRef string

	var account string
	var ramBytes int

	cmd := &cobra.Command{
		Use:     "buy-ram",
		Aliases: []string{"buyram"},
		Short:   "Buy ram for any EOS account.",
		Args:    cobra.ExactArgs(0),
		RunE: func(cmd *cobra.Command, _args []string) error {
			xcFactory := setup.UnwrapXc(cmd.Context())
			chainConfig := setup.UnwrapChain(cmd.Context())
			ctx := cmd.Context()
			_ = xcFactory
			_ = chainConfig
			if account == "" {
				return fmt.Errorf("must set --account")
			}
			if ramBytes <= 0 {
				return fmt.Errorf("must set --ram")
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
			tfArgs, err := xcbuilder.NewTransferArgs(fromAddress, xc.Address(fromAddress), xc.NewAmountBlockchainFromUint64(1))
			if err != nil {
				return fmt.Errorf("invalid transfer args: %v", err)
			}

			inputI, err := client.FetchTransferInput(ctx, tfArgs)
			if err != nil {
				return fmt.Errorf("failed to fetch transfer input: %v", err)
			}
			input := inputI.(*tx_input.TxInput)
			input.Timestamp = time.Now().Unix()

			refApi := eos.New(chainConfig.URL, chainConfig.DefaultHttpClient().Timeout)
			refApi.Header.Set("Content-Type", "application/json")

			fromAccount := input.FromAccount

			actionBuyRam, err := action.NewBuyRamBytes(fromAccount, account, uint32(ramBytes))
			if err != nil {
				return fmt.Errorf("failed to create buy ram action: %v", err)
			}

			eosTx := &eos.Transaction{Actions: []*eos.Action{}}
			eosTx.RefBlockNum = uint16(binary.BigEndian.Uint32(input.HeadBlockID[:4]))
			eosTx.RefBlockPrefix = binary.LittleEndian.Uint32(input.HeadBlockID[8:16])
			expiration := time.Unix(input.Timestamp, 0)
			expiration = expiration.Add(tx_input.ExpirationPeriod)
			eosTx.Expiration = eos.JSONTime{Time: expiration}
			eosTx.Actions = []*eos.Action{actionBuyRam}

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
			binaryTx := xctypes.NewBinaryTx(packedTrxBytes, [][]byte{})
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

	cmd.Flags().StringVar(&account, "account", "", "Account to buy ram for.")
	cmd.Flags().IntVar(&ramBytes, "ram", 0, "Amount of RAM to buy for the account.")
	return cmd
}
