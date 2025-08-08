package commands

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"strconv"

	xc "github.com/cordialsys/crosschain"
	"github.com/cordialsys/crosschain/cmd/xc/setup"
	"github.com/cordialsys/crosschain/factory/signer"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

func CmdFund() *cobra.Command {
	var contract string
	var amountHuman string
	var decimalsStr string
	var privateKeyRef string
	var format string
	var api string
	cmd := &cobra.Command{
		Use:   "fund [address]",
		Short: "Request funds from a crosschain node faucet.",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			xcFactory := setup.UnwrapXc(cmd.Context())
			chainConfig := setup.UnwrapChain(cmd.Context())
			address, err := inputAddressOrDerived(xcFactory, chainConfig, args, privateKeyRef, format)
			if err != nil {
				return err
			}

			decimals := int(chainConfig.Decimals)
			if decimalsStr != "" {
				decimals, err = strconv.Atoi(decimalsStr)
				if err != nil {
					return err
				}
			}
			assetId := contract
			if assetId == "" {
				assetId = string(chainConfig.Chain)
			}

			amountHuman, err := xc.NewAmountHumanReadableFromStr(amountHuman)
			if err != nil {
				return err
			}
			amount := amountHuman.ToBlockchain(int32(decimals))
			url := fmt.Sprintf("%s/chains/%s/assets/%s", api, chainConfig.Chain, assetId)

			requestBody := map[string]interface{}{
				"amount":  amount.String(),
				"address": address,
			}
			req, err := http.NewRequest("POST", url, bytes.NewBuffer([]byte(asJson(requestBody))))
			if err != nil {
				return fmt.Errorf("error creating request: %v", err)
			}
			req.Header.Set("Content-Type", "application/json")
			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				return fmt.Errorf("error sending request: %v", err)
			}
			body, err := io.ReadAll(resp.Body)
			if err != nil {
				return fmt.Errorf("error reading response: %v", err)
			}
			if resp.StatusCode != http.StatusOK {
				logrus.Error(string(body))
				return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
			}
			fmt.Println(string(body))

			return nil
		},
	}
	cmd.Flags().StringVar(&contract, "contract", "", "Contract to use to get funds for.")
	cmd.Flags().StringVar(&decimalsStr, "decimals", "", "decimals of the token, when using --contract.")
	cmd.Flags().StringVar(&api, "api", "http://127.0.0.1:10001", "API url to use for faucet.")
	cmd.Flags().StringVar(&amountHuman, "amount", "1", "Decimal-adjusted amount of funds to request.")
	cmd.Flags().StringVar(&privateKeyRef, "key", "env:"+signer.EnvPrivateKey, "Private key reference")
	cmd.Flags().StringVar(&format, "format", "", "Optional address format for chains that use multiple address formats")
	return cmd
}
