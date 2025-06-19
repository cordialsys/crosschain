package eostools

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"runtime"
	"time"

	"github.com/cordialsys/crosschain"
	xc "github.com/cordialsys/crosschain"
	xcbuilder "github.com/cordialsys/crosschain/builder"
	xctypes "github.com/cordialsys/crosschain/chain/crosschain/types"
	"github.com/cordialsys/crosschain/chain/eos/address"
	eos "github.com/cordialsys/crosschain/chain/eos/eos-go"
	"github.com/cordialsys/crosschain/chain/eos/eos-go/ecc"
	eostx "github.com/cordialsys/crosschain/chain/eos/tx"
	"github.com/cordialsys/crosschain/chain/eos/tx/action"
	"github.com/cordialsys/crosschain/chain/eos/tx_input"
	"github.com/cordialsys/crosschain/cmd/xc/setup"
	"github.com/cordialsys/crosschain/config"
	"github.com/cordialsys/crosschain/factory/signer"
	"github.com/spf13/cobra"
)

func jsonPrint(v interface{}) {
	json, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		panic(err)
	}
	fmt.Println(string(json))
}

func encode(v interface{}) string {
	buffer := bytes.NewBuffer(nil)
	err := eos.NewEncoder(buffer).Encode(v)
	NoError(err, "encode %T", v)

	return hex.EncodeToString(buffer.Bytes())
}

func NoError(err error, message string, args ...interface{}) {
	if err != nil {
		Quit(message+": "+err.Error(), args...)
	}
}

func Quit(message string, args ...interface{}) {
	fmt.Printf(message+"\n", args...)
	runtime.Goexit()
}

type EosTransfer struct {
	From     eos.AccountName `json:"from"`
	To       eos.AccountName `json:"to"`
	Quantity eos.Asset       `json:"quantity"`
	Memo     string          `json:"memo"`
}

func CmdTxTransferEOS() *cobra.Command {
	var inclusiveFee bool
	var feePayer bool
	var dryRun bool
	var fromSecretRef string
	var feePayerSecretRef string
	var previousAttempts []string

	cmd := &cobra.Command{
		Use:     "transfer <to> <amount>",
		Aliases: []string{"tf"},
		Short:   "Create and broadcast a new transaction transferring funds. The amount should be a decimal amount.",
		Args:    cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			xcFactory := setup.UnwrapXc(cmd.Context())
			chainConfig := setup.UnwrapChain(cmd.Context())
			ctx := cmd.Context()
			_ = xcFactory
			_ = chainConfig
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

			toAccount := "cordialsysbb"

			builder, err := address.NewAddressBuilder(chainConfig.Base())
			if err != nil {
				return err
			}

			fromAddress, err := builder.GetAddressFromPublicKey(publicKey)
			if err != nil {
				return err
			}
			amountHuman, _ := xc.NewAmountHumanReadableFromStr("0.1")
			amount := amountHuman.ToBlockchain(4)
			tfArgs, err := xcbuilder.NewTransferArgs(fromAddress, xc.Address(toAccount), amount)
			if err != nil {
				return fmt.Errorf("invalid transfer args: %v", err)
			}

			inputI, err := client.FetchTransferInput(ctx, tfArgs)
			if err != nil {
				return fmt.Errorf("failed to fetch transfer input: %v", err)
			}
			input := inputI.(*tx_input.TxInput)
			input.Timestamp = time.Now().Unix()

			refApi := eos.New("https://jungle4.cryptolions.io:443")
			refApi.Header.Set("Content-Type", "application/json")

			xapi := eos.New("https://jungle4.cryptolions.io:443")
			xapi.Header.Set("Content-Type", "application/json")

			fromAccount := input.FromAccount

			transferQuantity, err := eos.NewAssetFromString("0.1000 EOS")
			if err != nil {
				return fmt.Errorf("failed to create transfer quantity: %v", err)
			}
			_ = transferQuantity

			theAction, err := action.NewTransfer(fromAccount, toAccount, amountHuman, 4, "eosio.token", "EOS", "")
			if err != nil {
				return fmt.Errorf("failed to create transfer action: %v", err)
			}
			_ = theAction

			eosTx := &eos.Transaction{Actions: []*eos.Action{}}
			eosTx.RefBlockNum = uint16(binary.BigEndian.Uint32(input.HeadBlockID[:4]))
			eosTx.RefBlockPrefix = binary.LittleEndian.Uint32(input.HeadBlockID[8:16])
			expiration := time.Unix(input.Timestamp, 0)
			expiration = expiration.Add(tx_input.ExpirationPeriod)
			eosTx.Expiration = eos.JSONTime{Time: expiration}

			signedTx := eos.NewSignedTransaction(eosTx)

			for i := 0; i < 100; i++ {
				// lol this is our 'nonce'
				expiration = expiration.Add(1 * time.Second)
				eosTx.Expiration = eos.JSONTime{Time: expiration}

				// sigDigest, err := eostx.Sighash(eosTx, input.ChainID)
				// if err != nil {
				// 	return fmt.Errorf("failed to sighash transaction: %v", err)
				// }
				sigDigest := []byte{}
				sig, err := mainSigner.Sign(&crosschain.SignatureRequest{
					Payload: sigDigest,
				})
				if err != nil {
					return fmt.Errorf("failed to sign transaction: %v", err)
				}
				canonicalSigMaybe := eostx.SwapRecoveryByte(sig.Signature)
				fmt.Printf("-- %d canonicalSigMaybe: %s (%d)\n", i, hex.EncodeToString(canonicalSigMaybe), len(canonicalSigMaybe))
				if eostx.IsCanonical(canonicalSigMaybe) {
					withPrefix := append([]byte{byte(ecc.CurveK1)}, canonicalSigMaybe...)
					sigFormatted, err := ecc.NewSignatureFromData(withPrefix)
					if err != nil {
						return fmt.Errorf("failed to format signature: %v", err)
					}
					signedTx = eos.NewSignedTransaction(eosTx)
					signedTx.Signatures = []ecc.Signature{sigFormatted}
					break
				} else {
					// keep trying ...
				}
			}

			packedTrx, err := signedTx.Pack(eos.CompressionNone)
			NoError(err, "pack transaction")
			packedTrx.Compression = false

			trxID, err := packedTrx.ID()
			NoError(err, "transaction id")
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

			out, err := refApi.PushRawTransactionRaw(ctx, serializedTx)
			if err != nil {
				return fmt.Errorf("failed to send transaction: %v", err)
			}
			jsonPrint(out)

			return nil
		},
	}
	cmd.Flags().StringVar(&fromSecretRef, "from", "env:"+signer.EnvPrivateKey, "Secret reference for the from-address private key")
	cmd.Flags().StringVar(&feePayerSecretRef, "fee-payer-secret", "env:"+signer.EnvPrivateKeyFeePayer, "Secret reference for the fee-payer address private key")
	cmd.Flags().String("contract", "", "Contract address of asset to send, if applicable")
	cmd.Flags().String("decimals", "", "Decimals of the token, when using --contract.")
	cmd.Flags().String("memo", "", "Set a memo for the transfer.")
	cmd.Flags().BoolVar(&feePayer, "fee-payer", false, "Use another address to pay the fee for the transaction (uses --fee-payer-secret)")
	cmd.Flags().String("priority", "", "Apply a priority for the transaction fee ('low', 'market', 'aggressive', 'very-aggressive', or any positive decimal number)")
	cmd.Flags().Duration("timeout", 1*time.Minute, "Amount of time to wait for transaction to confirm on chain.")
	cmd.Flags().BoolVar(&inclusiveFee, "inclusive-fee", false, "Include the fee in the transfer amount.")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Dry run the transaction, printing it, but not submitting it.")
	cmd.Flags().StringSliceVar(&previousAttempts, "previous", []string{}, "List of transaction hashes that have been attempted and may still be in the mempool.")
	return cmd
}
