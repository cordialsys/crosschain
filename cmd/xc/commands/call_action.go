package commands

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	xc "github.com/cordialsys/crosschain"
	"github.com/cordialsys/crosschain/builder"
	xccall "github.com/cordialsys/crosschain/call"
	solanacall "github.com/cordialsys/crosschain/chain/solana/call"
	xclient "github.com/cordialsys/crosschain/client"
	"github.com/cordialsys/crosschain/cmd/xc/setup"
	"github.com/cordialsys/crosschain/config"
	"github.com/cordialsys/crosschain/factory"
	"github.com/cordialsys/crosschain/factory/drivers"
	"github.com/cordialsys/crosschain/factory/signer"
	fsigner "github.com/cordialsys/crosschain/factory/signer"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

func CmdCallTx() *cobra.Command {
	var privateKeyRef string
	var signerSecretRefs []string
	var nonceAccount string
	var submit bool
	var timeout time.Duration
	var methodStr string

	cmd := &cobra.Command{
		Use:   "call-tx <call-payload-json|@payload-file|hex-encoded-solana-tx>",
		Short: "Sign and optionally broadcast a chain call transaction.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			xcFactory := setup.UnwrapXc(ctx)
			chainConfig := setup.UnwrapChain(ctx)

			method := defaultCallMethod(chainConfig.Driver, methodStr)
			if !method.Valid() {
				return fmt.Errorf("invalid call method: %s", method)
			}

			mainSigner, mainAddress, err := SignerAndAddress(xcFactory, chainConfig, privateKeyRef)
			if err != nil {
				return err
			}

			signerCollection := fsigner.NewCollection()
			signerCollection.AddMainSigner(mainSigner, mainAddress)
			signingAddresses := []xc.Address{mainAddress}

			for _, signerSecretRef := range signerSecretRefs {
				auxSigner, auxAddress, err := SignerAndAddress(xcFactory, chainConfig, signerSecretRef)
				if err != nil {
					return err
				}
				signerCollection.AddAuxSigner(auxSigner, auxAddress)
				signingAddresses = append(signingAddresses, auxAddress)
			}

			payload, err := callPayloadFromArg(chainConfig.Base(), args[0])
			if err != nil {
				return fmt.Errorf("could not parse call payload: %v", err)
			}
			callTx, err := drivers.NewCall(chainConfig.Base(), method, payload, signingAddresses)
			if err != nil {
				return fmt.Errorf("could not build call transaction: %v", err)
			}

			callOptions := []builder.BuilderOption{}
			if nonceAccount != "" {
				callOptions = append(callOptions, builder.OptionNonceAccount(nonceAccount))
			}
			callArgs, err := builder.NewCallArgs(chainConfig.Base(), callOptions...)
			if err != nil {
				return fmt.Errorf("could not build call arguments: %v", err)
			}

			rpcClient, err := xcFactory.NewClient(chainConfig)
			if err != nil {
				return fmt.Errorf("could not load client: %v", err)
			}
			callClient, ok := rpcClient.(xclient.CallClient)
			if !ok {
				return fmt.Errorf("chain %s does not support call transactions", chainConfig.Chain)
			}

			input, err := callClient.FetchCallInput(ctx, callTx, callArgs)
			if err != nil {
				return fmt.Errorf("could not fetch call input: %v", err)
			}

			tx, err := PrepareCallForSubmit(callTx, input, signerCollection)
			if err != nil {
				return fmt.Errorf("could not prepare call transaction: %v", err)
			}

			if payloadSetter, ok := input.(xc.TxInputWithCall); ok {
				if payload, ok := callTx.GetPayload(); ok {
					if err := payloadSetter.SetCall(payload); err != nil {
						return fmt.Errorf("could not sync call input: %v", err)
					}
				}
			}

			serialized, err := tx.Serialize()
			if err != nil {
				return fmt.Errorf("could not serialize signed call transaction: %v", err)
			}

			if submit {
				if err := SubmitTransaction(chainConfig.Chain, rpcClient, tx, timeout); err != nil {
					return fmt.Errorf("could not submit call transaction: %v", err)
				}
				logrus.WithField("hash", tx.Hash()).Info("submitted tx")
			}

			fmt.Println(asJson(map[string]any{
				"hash":             tx.Hash(),
				"method":           method,
				"signingAddresses": signingAddresses,
				"submitted":        submit,
				"transaction":      hex.EncodeToString(serialized),
			}))
			return nil
		},
	}

	cmd.Flags().StringVar(&privateKeyRef, "key", "env:"+signer.EnvPrivateKey, "Main signer private key reference")
	cmd.Flags().StringArrayVar(&signerSecretRefs, "signer-secret", []string{}, "Additional signer private key reference; may be repeated")
	cmd.Flags().StringVar(&nonceAccount, "nonce-account", "", "Durable nonce account address for call input conflict tracking")
	cmd.Flags().BoolVar(&submit, "submit", false, "Broadcast the signed call transaction")
	cmd.Flags().DurationVar(&timeout, "timeout", 1*time.Minute, "Amount of time to wait for transaction submission")
	cmd.Flags().StringVar(&methodStr, "method", "", "Call method; defaults by chain")
	return cmd
}

func defaultCallMethod(driver xc.Driver, methodStr string) xccall.Method {
	if methodStr != "" {
		return xccall.Method(methodStr)
	}
	switch driver {
	case xc.DriverEVM, xc.DriverEVMLegacy:
		return xccall.EthSendTransaction
	case xc.DriverSolana:
		return xccall.SolanaSignTransaction
	case xc.DriverCanton:
		return xccall.OfferAccept
	default:
		return ""
	}
}

func callPayloadFromArg(chain *xc.ChainBaseConfig, arg string) (json.RawMessage, error) {
	payload := []byte(arg)
	if strings.HasPrefix(arg, "@") {
		bz, err := os.ReadFile(strings.TrimPrefix(arg, "@"))
		if err != nil {
			return nil, err
		}
		payload = bz
	}
	payload = []byte(strings.TrimSpace(string(payload)))

	if json.Valid(payload) {
		return json.RawMessage(payload), nil
	}

	if chain.Driver != xc.DriverSolana {
		return nil, fmt.Errorf("expected JSON payload for %s call", chain.Driver)
	}

	txBytes, err := hex.DecodeString(strings.TrimPrefix(string(payload), "0x"))
	if err != nil {
		return nil, fmt.Errorf("expected JSON payload or hex-encoded Solana transaction: %w", err)
	}
	return json.Marshal(solanacall.Call{Transaction: txBytes})
}

func SignerAndAddress(xcFactory *factory.Factory, chainConfig *xc.ChainConfig, secretRef string) (*signer.Signer, xc.Address, error) {
	privateKeyInput, err := config.GetSecret(secretRef)
	if err != nil {
		return nil, "", fmt.Errorf("could not get secret %s: %v", secretRef, err)
	}
	if privateKeyInput == "" {
		return nil, "", fmt.Errorf("secret reference %s loaded an empty value", secretRef)
	}

	chainOpts := ChainAddressOptions(chainConfig)
	xcSigner, err := xcFactory.NewSigner(chainConfig.Base(), privateKeyInput, chainOpts...)
	if err != nil {
		return nil, "", fmt.Errorf("could not import private key %s: %v", secretRef, err)
	}
	publicKey, err := xcSigner.PublicKey()
	if err != nil {
		return nil, "", fmt.Errorf("could not create public key for %s: %v", secretRef, err)
	}
	addressBuilder, err := xcFactory.NewAddressBuilder(chainConfig.Base(), chainOpts...)
	if err != nil {
		return nil, "", fmt.Errorf("could not create address builder: %v", err)
	}
	address, err := addressBuilder.GetAddressFromPublicKey(publicKey)
	if err != nil {
		return nil, "", fmt.Errorf("could not derive address for %s: %v", secretRef, err)
	}
	return xcSigner, address, nil
}

func PrepareCallForSubmit(callTx xc.TxCall, input xc.CallTxInput, signerCollection *fsigner.Collection) (xc.Tx, error) {
	if err := callTx.SetInput(input); err != nil {
		return nil, fmt.Errorf("could not set call input: %v", err)
	}

	signatures := []*xc.SignatureResponse{}
	sighashes, err := callTx.Sighashes()
	if err != nil {
		return nil, fmt.Errorf("could not create payloads to sign: %v", err)
	}
	if len(sighashes) == 0 {
		return nil, fmt.Errorf("call transaction produced no sighashes")
	}
	for _, sighash := range sighashes {
		signature, err := signerCollection.Sign(sighash.Signer, sighash.Payload)
		if err != nil {
			return nil, err
		}
		signatures = append(signatures, signature)
	}
	if err := callTx.SetSignatures(signatures...); err != nil {
		return nil, fmt.Errorf("could not add signature(s): %v", err)
	}

	if txMoreSigs, ok := callTx.(xc.TxAdditionalSighashes); ok {
		for {
			additionalSighashes, err := txMoreSigs.AdditionalSighashes()
			if err != nil {
				return nil, fmt.Errorf("could not create additional payloads to sign: %v", err)
			}
			if len(additionalSighashes) == 0 {
				break
			}
			for _, additionalSighash := range additionalSighashes {
				signature, err := signerCollection.Sign(additionalSighash.Signer, additionalSighash.Payload)
				if err != nil {
					return nil, err
				}
				signatures = append(signatures, signature)
			}
			if err := callTx.SetSignatures(signatures...); err != nil {
				return nil, fmt.Errorf("could not add additional signature(s): %v", err)
			}
		}
	}

	return callTx, nil
}
