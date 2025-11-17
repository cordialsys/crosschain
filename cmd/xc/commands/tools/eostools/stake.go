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
	"github.com/cordialsys/crosschain/chain/eos/tx_input"
	xctypes "github.com/cordialsys/crosschain/client/types"
	"github.com/cordialsys/crosschain/cmd/xc/setup"
	"github.com/cordialsys/crosschain/config"
	"github.com/cordialsys/crosschain/factory/signer"
	"github.com/spf13/cobra"
)

func CmdTxStake() *cobra.Command {
	var dryRun bool
	var fromSecretRef string

	var account string
	var net, cpu bool
	var amountStr string
	var unstake bool

	cmd := &cobra.Command{
		Use:   "stake",
		Short: "Stake CPU or NET to any EOS account.  Note that the EOS will be transferred - the destination account will take possession of the EOS.",
		Args:  cobra.ExactArgs(0),
		RunE: func(cmd *cobra.Command, _args []string) error {
			xcFactory := setup.UnwrapXc(cmd.Context())
			chainConfig := setup.UnwrapChain(cmd.Context())
			ctx := cmd.Context()
			_ = xcFactory
			_ = chainConfig
			if account == "" {
				return fmt.Errorf("must set --account")
			}
			if !net && !cpu {
				return fmt.Errorf("must set --net or --cpu, or both")
			}
			amountHuman, err := xc.NewAmountHumanReadableFromStr(amountStr)
			if err != nil {
				return fmt.Errorf("invalid amount: %v", err)
			}
			amount := amountHuman.ToBlockchain(action.Decimals)

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

			netAmount := xc.NewAmountBlockchainFromUint64(0)
			cpuAmount := xc.NewAmountBlockchainFromUint64(0)
			div := 0
			if cpu {
				div++
				cpuAmount = amount
			}
			if net {
				div++
				netAmount = amount
			}
			if div > 0 {
				denom := xc.NewAmountBlockchainFromUint64(uint64(div))
				cpuAmount = cpuAmount.Div(&denom)
				netAmount = netAmount.Div(&denom)
			}

			actionStake, err := action.NewDelegateBW(fromAccount, account, cpuAmount, netAmount, true)
			if err != nil {
				return fmt.Errorf("failed to create stake action: %v", err)
			}
			if unstake {
				actionStake, err = action.NewUnDelegateBW(fromAccount, account, cpuAmount, netAmount)
				if err != nil {
					return fmt.Errorf("failed to create unstake action: %v", err)
				}
			}

			eosTx := &eos.Transaction{Actions: []*eos.Action{}}
			eosTx.RefBlockNum = uint16(binary.BigEndian.Uint32(input.HeadBlockID[:4]))
			eosTx.RefBlockPrefix = binary.LittleEndian.Uint32(input.HeadBlockID[8:16])
			expiration := time.Unix(input.Timestamp, 0)
			expiration = expiration.Add(tx_input.ExpirationPeriod)
			eosTx.Expiration = eos.JSONTime{Time: expiration}
			eosTx.Actions = []*eos.Action{actionStake}

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

	cmd.Flags().StringVar(&account, "account", "", "Account to stake to.")
	cmd.Flags().BoolVar(&net, "net", false, "Stake to net.")
	cmd.Flags().BoolVar(&cpu, "cpu", false, "Stake to cpu.")
	cmd.Flags().StringVar(&amountStr, "amount", "", "Amount to stake.")
	cmd.Flags().BoolVar(&unstake, "unstake", false, "Unstake instead of stake.")
	return cmd
}
