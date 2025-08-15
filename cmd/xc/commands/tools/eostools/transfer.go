package eostools

import (
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"

	"github.com/cordialsys/crosschain"
	xc "github.com/cordialsys/crosschain"
	xcbuilder "github.com/cordialsys/crosschain/builder"
	xctypes "github.com/cordialsys/crosschain/chain/crosschain/types"
	"github.com/cordialsys/crosschain/chain/eos/address"
	"github.com/cordialsys/crosschain/chain/eos/builder/action"
	eos "github.com/cordialsys/crosschain/chain/eos/eos-go"
	"github.com/cordialsys/crosschain/chain/eos/eos-go/ecc"
	eostx "github.com/cordialsys/crosschain/chain/eos/tx"
	"github.com/cordialsys/crosschain/chain/eos/tx_input"
	"github.com/cordialsys/crosschain/cmd/xc/setup"
	"github.com/cordialsys/crosschain/config"
	"github.com/cordialsys/crosschain/factory/signer"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

func jsonPrint(v interface{}) {
	json, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		panic(err)
	}
	fmt.Println(string(json))
}

func signEosTx(mainSigner *signer.Signer, eosTx *eos.Transaction, input *tx_input.TxInput) (*eos.SignedTransaction, error) {
	expiration := eosTx.Expiration.Time
	signedTx := eos.NewSignedTransaction(eosTx)
	for i := 0; i < 100; i++ {
		expiration = expiration.Add(1 * time.Second)
		eosTx.Expiration = eos.JSONTime{Time: expiration}

		sigDigest, err := eostx.Sighash(eosTx, input.ChainID)
		if err != nil {
			return nil, fmt.Errorf("failed to create sighash: %v", err)
		}
		sig, err := mainSigner.Sign(&crosschain.SignatureRequest{
			Payload: sigDigest,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to sign transaction: %v", err)
		}
		canonicalSigMaybe := eostx.SwapRecoveryByte(sig.Signature)
		logrus.WithFields(logrus.Fields{
			"i":         i,
			"canonical": eostx.IsCanonical(canonicalSigMaybe),
			"signature": hex.EncodeToString(sig.Signature),
		}).Info("signed transaction")
		if eostx.IsCanonical(canonicalSigMaybe) {
			withPrefix := append([]byte{byte(ecc.CurveK1)}, canonicalSigMaybe...)
			sigFormatted, err := ecc.NewSignatureFromData(withPrefix)
			if err != nil {
				return nil, fmt.Errorf("failed to format signature: %v", err)
			}
			signedTx = eos.NewSignedTransaction(eosTx)
			signedTx.Signatures = []ecc.Signature{sigFormatted}
			break
		} else {
			// keep trying ...
		}
	}
	return signedTx, nil
}

func CmdTxTransferEOS() *cobra.Command {
	var dryRun bool
	var fromSecretRef string
	var memo string

	cmd := &cobra.Command{
		Use:     "transfer <to> <amount>",
		Aliases: []string{"tf"},
		Short:   "Send an EOS transfer.",
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

			toAccount := args[0]

			builder, err := address.NewAddressBuilder(chainConfig.Base())
			if err != nil {
				return err
			}

			fromAddress, err := builder.GetAddressFromPublicKey(publicKey)
			if err != nil {
				return err
			}
			amountHuman, _ := xc.NewAmountHumanReadableFromStr(args[1])
			amount := amountHuman.ToBlockchain(4)
			tfArgs, err := xcbuilder.NewTransferArgs(chainConfig.Base(), fromAddress, xc.Address(toAccount), amount)
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

			actionTf, err := action.NewTransfer(fromAccount, toAccount, amount, 4, "eosio.token", "EOS", memo)
			if err != nil {
				return fmt.Errorf("failed to create transfer action: %v", err)
			}

			eosTx := &eos.Transaction{Actions: []*eos.Action{}}
			eosTx.RefBlockNum = uint16(binary.BigEndian.Uint32(input.HeadBlockID[:4]))
			eosTx.RefBlockPrefix = binary.LittleEndian.Uint32(input.HeadBlockID[8:16])
			expiration := time.Unix(input.Timestamp, 0)
			expiration = expiration.Add(tx_input.ExpirationPeriod)
			eosTx.Expiration = eos.JSONTime{Time: expiration}
			eosTx.Actions = []*eos.Action{actionTf}

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
	cmd.Flags().StringVar(&fromSecretRef, "from", "env:"+signer.EnvPrivateKey, "Secret reference for the from-address private key")
	cmd.Flags().StringVar(&memo, "memo", "", "Set a memo for the transfer.")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Dry run the transaction, printing it, but not submitting it.")
	return cmd
}
