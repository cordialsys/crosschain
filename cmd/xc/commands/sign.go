package commands

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"strings"

	xc "github.com/cordialsys/crosschain"
	xcaddress "github.com/cordialsys/crosschain/address"
	"github.com/cordialsys/crosschain/cmd/xc/setup"
	"github.com/cordialsys/crosschain/config"
	"github.com/cordialsys/crosschain/factory/signer"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

func CmdSign() *cobra.Command {
	var fromSecretRef string
	// var format string
	var doSha256 bool

	cmd := &cobra.Command{
		Use:   "sign <payload>",
		Short: "Sign a hex or base64 payload using a private key for a particular chain.",
		Args:  cobra.RangeArgs(0, 1),
		RunE: func(cmd *cobra.Command, args []string) error {
			var err error
			xcFactory := setup.UnwrapXc(cmd.Context())
			chainConfig := setup.UnwrapChain(cmd.Context())

			var payload []byte
			var payloadS string
			if len(args) == 0 {
				bz, err := io.ReadAll(os.Stdin)
				if err != nil {
					return fmt.Errorf("could not read payload from stdin: %v", err)
				}
				payload = bz
			} else {
				payloadS = args[0]
			}
			if len(payloadS) > 0 {
				payload, err = hex.DecodeString(strings.TrimPrefix(payloadS, "0x"))
				if err != nil {
					// try base64
					payload, err = base64.StdEncoding.DecodeString(payloadS)
					if err != nil {
						return fmt.Errorf("could not decode payload: %v", err)
					}
				}
			}

			if len(payload) == 0 {
				panic("requested to sign empty payload")
			}

			logrus.WithField("payload", hex.EncodeToString(payload)).WithField("algorithm", chainConfig.Driver.SignatureAlgorithms()).Info("signing payload")
			if doSha256 {
				hash := sha256.Sum256(payload)
				payload = hash[:]
				logrus.WithField("payload", hex.EncodeToString(payload)).Info("hashed payload")
			}

			addressArgs := []xcaddress.AddressOption{}
			algorithm, _ := cmd.Flags().GetString("algorithm")
			if algorithm != "" {
				addressArgs = append(addressArgs, xcaddress.OptionAlgorithm(xc.SignatureType(algorithm)))
			}

			privateKeyInput, err := config.GetSecret(fromSecretRef)
			if err != nil {
				return fmt.Errorf("could not get from-address secret: %v", err)
			}
			if privateKeyInput == "" {
				return fmt.Errorf("must set env %s", signer.EnvPrivateKey)
			}
			mainSigner, err := xcFactory.NewSigner(chainConfig.Base(), privateKeyInput, addressArgs...)
			if err != nil {
				return fmt.Errorf("could not import private key: %v", err)
			}

			signature, err := mainSigner.Sign(&xc.SignatureRequest{
				Payload: payload,
			})
			if err != nil {
				panic(err)
			}
			fmt.Println(hex.EncodeToString(signature.Signature))
			return nil
		},
	}
	cmd.Flags().StringVar(&fromSecretRef, "from", "env:"+signer.EnvPrivateKey, "Secret reference for the from-address private key")
	cmd.Flags().BoolVar(&doSha256, "sha256", false, "Hash the payload before signing")
	return cmd
}
