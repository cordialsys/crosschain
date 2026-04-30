package commands

import (
	"encoding/json"
	"fmt"
	"time"

	xc "github.com/cordialsys/crosschain"
	xccall "github.com/cordialsys/crosschain/call"
	xclient "github.com/cordialsys/crosschain/client"
	"github.com/cordialsys/crosschain/cmd/xc/setup"
	"github.com/cordialsys/crosschain/config"
	"github.com/cordialsys/crosschain/factory/drivers"
	"github.com/cordialsys/crosschain/factory/signer"
	fsigner "github.com/cordialsys/crosschain/factory/signer"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

func CmdOfferAccept() *cobra.Command {
	return newCallActionCommand(
		"offer-accept <contract-id> [address]",
		"Accept a pending offer by contract id.",
		xccall.OfferAccept,
	)
}

func CmdSettlementComplete() *cobra.Command {
	return newCallActionCommand(
		"settlement-complete <contract-id> [address]",
		"Complete a pending settlement by contract id.",
		xccall.SettlementComplete,
	)
}

func newCallActionCommand(use string, short string, method xccall.Method) *cobra.Command {
	var privateKeyRef string
	var format string
	var timeout time.Duration

	cmd := &cobra.Command{
		Use:   use,
		Short: short,
		Args:  cobra.RangeArgs(1, 2),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			xcFactory := setup.UnwrapXc(ctx)
			chainConfig := setup.UnwrapChain(ctx)

			contractID := args[0]
			address, err := inputAddressOrDerived(xcFactory, chainConfig, args[1:], privateKeyRef, format)
			if err != nil {
				return err
			}

			privateKeyInput, err := config.GetSecret(privateKeyRef)
			if err != nil {
				return fmt.Errorf("could not get secret: %v", err)
			}
			if privateKeyInput == "" {
				return fmt.Errorf("must set env %s", signer.EnvPrivateKey)
			}

			xcSigner, err := xcFactory.NewSigner(chainConfig.Base(), privateKeyInput)
			if err != nil {
				return fmt.Errorf("could not import private key: %v", err)
			}
			signerCollection := fsigner.NewCollection()
			signerCollection.AddMainSigner(xcSigner, address)

			rpcClient, err := xcFactory.NewClient(chainConfig)
			if err != nil {
				return fmt.Errorf("could not load client: %v", err)
			}
			callClient, ok := rpcClient.(xclient.CallClient)
			if !ok {
				return fmt.Errorf("chain %s does not support call actions", chainConfig.Chain)
			}

			payload, err := marshalCallPayload(method, contractID)
			if err != nil {
				return err
			}
			callTx, err := drivers.NewCall(chainConfig.Base(), method, payload, address)
			if err != nil {
				return fmt.Errorf("could not build call transaction: %v", err)
			}

			input, err := callClient.FetchCallInput(ctx, callTx)
			if err != nil {
				return fmt.Errorf("could not fetch call input: %v", err)
			}

			tx, err := prepareCallForSubmit(callTx, input, signerCollection)
			if err != nil {
				return fmt.Errorf("could not prepare call transaction: %v", err)
			}

			if err := SubmitTransaction(chainConfig.Chain, rpcClient, tx, timeout); err != nil {
				return fmt.Errorf("could not submit call transaction: %v", err)
			}

			logrus.WithField("hash", tx.Hash()).Info("submitted tx")
			fmt.Println(asJson(map[string]any{
				"hash":       tx.Hash(),
				"method":     method,
				"address":    address,
				"contractId": contractID,
			}))
			return nil
		},
	}

	cmd.Flags().StringVar(&privateKeyRef, "key", "env:"+signer.EnvPrivateKey, "Private key reference")
	cmd.Flags().StringVar(&format, "format", "", "Optional address format for chains that use multiple address formats")
	cmd.Flags().DurationVar(&timeout, "timeout", 1*time.Minute, "Amount of time to wait for transaction submission.")
	return cmd
}

func marshalCallPayload(method xccall.Method, contractID string) ([]byte, error) {
	switch method {
	case xccall.OfferAccept:
		return json.Marshal(xccall.OfferAcceptCall{ContractID: contractID})
	case xccall.SettlementComplete:
		return json.Marshal(xccall.SettlementCompleteCall{ContractID: contractID})
	default:
		return nil, fmt.Errorf("unsupported call method %q", method)
	}
}

func prepareCallForSubmit(callTx xc.TxCall, input xc.CallTxInput, signerCollection *fsigner.Collection) (xc.Tx, error) {
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
