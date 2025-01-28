package commands

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"

	xc "github.com/cordialsys/crosschain"
	"github.com/cordialsys/crosschain/chain/crosschain"
	xcclient "github.com/cordialsys/crosschain/client"
	"github.com/cordialsys/crosschain/cmd/xc/setup"
	"github.com/cordialsys/crosschain/factory"
	"github.com/cordialsys/crosschain/factory/signer"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

func inputAddressOrDerived(xcFactory *factory.Factory, chainConfig *xc.ChainConfig, args []string) (xc.Address, error) {
	if len(args) > 0 {
		return xc.Address(args[0]), nil
	}
	privateKeyInput := signer.ReadPrivateKeyEnv()
	if privateKeyInput == "" {
		return "", fmt.Errorf("must provide [address] as input, set env %s for it to be derived", signer.EnvPrivateKey)
	}
	signer, err := xcFactory.NewSigner(chainConfig, privateKeyInput)
	if err != nil {
		return "", fmt.Errorf("could not import private key: %v", err)
	}

	publicKey, err := signer.PublicKey()
	if err != nil {
		return "", fmt.Errorf("could not create public key: %v", err)
	}
	addressBuilder, err := xcFactory.NewAddressBuilder(chainConfig)
	if err != nil {
		return "", fmt.Errorf("could not create address builder: %v", err)
	}

	from, err := addressBuilder.GetAddressFromPublicKey(publicKey)
	if err != nil {
		return "", fmt.Errorf("could not derive address: %v", err)
	}
	return from, nil
}

func asJson(data any) string {
	bz, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		panic(err)
	}
	return string(bz)
}

func CmdRpcBalance() *cobra.Command {
	var contract string
	cmd := &cobra.Command{
		Use:   "balance [address]",
		Short: "Check balance of an asset.  Reported as big integer, not accounting for any decimals.",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			contract, _ := cmd.Flags().GetString("contract")
			xcFactory := setup.UnwrapXc(cmd.Context())
			chainConfig := setup.UnwrapChain(cmd.Context())
			address, err := inputAddressOrDerived(xcFactory, chainConfig, args)
			if err != nil {
				return err
			}

			client, err := xcFactory.NewClient(assetConfig(chainConfig, contract, 0))
			if err != nil {
				return err
			}

			balance, err := client.FetchBalance(context.Background(), address)
			if err != nil {
				return fmt.Errorf("could not fetch balance for address %s: %v", address, err)
			}

			fmt.Println(balance.String())

			return nil
		},
	}
	cmd.Flags().StringVar(&contract, "contract", "", "Optional contract of token asset")
	return cmd
}

func CmdTxInput() *cobra.Command {
	var addressTo string
	var contract string
	cmd := &cobra.Command{
		Use:     "tx-input [address]",
		Aliases: []string{"input"},
		Short:   "Check inputs for a new transaction.",
		Args:    cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			addressTo, _ := cmd.Flags().GetString("to")
			contract, _ := cmd.Flags().GetString("contract")
			xcFactory := setup.UnwrapXc(cmd.Context())
			chainConfig := setup.UnwrapChain(cmd.Context())
			fromAddress, err := inputAddressOrDerived(xcFactory, chainConfig, args)
			if err != nil {
				return err
			}

			client, err := xcFactory.NewClient(assetConfig(chainConfig, contract, 0))
			if err != nil {
				return err
			}

			input, err := client.FetchLegacyTxInput(context.Background(), fromAddress, xc.Address(addressTo))
			if err != nil {
				return fmt.Errorf("could not fetch transaction input: %v", err)
			}

			fmt.Println(asJson(input))

			return nil
		},
	}
	cmd.Flags().StringVar(&contract, "contract", "", "Optional contract of token asset")
	cmd.Flags().StringVar(&addressTo, "to", "", "Optional destination address")
	return cmd
}

func CmdTxInfo() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "tx-info <hash>",
		Aliases: []string{"tx"},
		Short:   "Check an existing transaction on chain.",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			xcFactory := setup.UnwrapXc(cmd.Context())
			chainConfig := setup.UnwrapChain(cmd.Context())
			hash := args[0]

			client, err := xcFactory.NewClient(assetConfig(chainConfig, "", 0))
			if err != nil {
				return fmt.Errorf("could not load client: %v", err)
			}

			txInfo, err := client.FetchTxInfo(context.Background(), xc.TxHash(hash))
			if err != nil {
				return fmt.Errorf("could not fetch tx info: %v", err)
			}

			fmt.Println(asJson(txInfo))

			return nil
		},
	}
	return cmd
}

func CmdTxTransfer() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "transfer <to> <amount>",
		Aliases: []string{"tf"},
		Short:   "Create and broadcast a new transaction transferring funds. The amount should be a decimal amount.",
		Args:    cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			xcFactory := setup.UnwrapXc(cmd.Context())
			chainConfig := setup.UnwrapChain(cmd.Context())

			contract, err := cmd.Flags().GetString("contract")
			if err != nil {
				return err
			}

			decimalsStr, err := cmd.Flags().GetString("decimals")
			if err != nil {
				return err
			}

			memo, err := cmd.Flags().GetString("memo")
			if err != nil {
				return err
			}

			timeout, err := cmd.Flags().GetDuration("timeout")
			if err != nil {
				return err
			}

			if decimalsStr == "" && contract != "" {
				return fmt.Errorf("must set --decimals if using --contract")
			}

			toWalletAddress := args[0]
			transferredAmount := args[1]

			decimals := chainConfig.GetDecimals()
			if contract != "" {
				parsed, err := strconv.ParseUint(decimalsStr, 10, 32)
				if err != nil {
					return fmt.Errorf("invalid decimals: %v", err)
				}
				decimals = int32(parsed)
			}

			privateKeyInput := signer.ReadPrivateKeyEnv()
			if privateKeyInput == "" {
				return fmt.Errorf("must set env %s", signer.EnvPrivateKey)
			}

			client, err := xcFactory.NewClient(assetConfig(chainConfig, contract, decimals))
			if err != nil {
				return fmt.Errorf("could not load client: %v", err)
			}

			transferredAmountHuman, err := xc.NewAmountHumanReadableFromStr(transferredAmount)
			if err != nil {
				return err
			}

			amountBlockchain := transferredAmountHuman.ToBlockchain(decimals)

			signer, err := xcFactory.NewSigner(chainConfig, privateKeyInput)
			if err != nil {
				return fmt.Errorf("could not import private key: %v", err)
			}

			publicKey, err := signer.PublicKey()
			if err != nil {
				return fmt.Errorf("could not create public key: %v", err)
			}

			addressBuilder, err := xcFactory.NewAddressBuilder(chainConfig)
			if err != nil {
				return fmt.Errorf("could not create address builder: %v", err)
			}

			from, err := addressBuilder.GetAddressFromPublicKey(publicKey)
			if err != nil {
				return fmt.Errorf("could not derive address: %v", err)
			}
			logrus.WithField("address", from).Info("sending from")

			input, err := client.FetchLegacyTxInput(context.Background(), from, xc.Address(toWalletAddress))
			if err != nil {
				return fmt.Errorf("could not fetch transfer input: %v", err)
			}

			if inputWithPublicKey, ok := input.(xc.TxInputWithPublicKey); ok {
				inputWithPublicKey.SetPublicKey(publicKey)
				logrus.WithField("public_key", hex.EncodeToString(publicKey)).Debug("added public key to transfer input")
			}

			if inputWithAmount, ok := input.(xc.TxInputWithAmount); ok {
				inputWithAmount.SetAmount(amountBlockchain)
			}

			if memo != "" {
				if txInputWithMemo, ok := input.(xc.TxInputWithMemo); ok {
					txInputWithMemo.SetMemo(memo)
				} else {
					return fmt.Errorf("cannot set memo; chain driver currently does not support memos")
				}
			}
			bz, _ := json.Marshal(input)
			logrus.WithField("input", string(bz)).Debug("transfer input")

			// create tx
			// (no network, no private key needed)
			builder, err := xcFactory.NewTxBuilder(assetConfig(chainConfig, contract, decimals))
			if err != nil {
				return fmt.Errorf("could not load tx-builder: %v", err)
			}

			tx, err := builder.NewTransfer(from, xc.Address(toWalletAddress), amountBlockchain, input)
			if err != nil {
				return fmt.Errorf("could not build transfer: %v", err)
			}

			sighashes, err := tx.Sighashes()
			if err != nil {
				return fmt.Errorf("could not create payloads to sign: %v", err)
			}

			// sign
			signatures := []xc.TxSignature{}
			for _, sighash := range sighashes {
				// sign the tx sighash(es)
				signature, err := signer.Sign(sighash)
				if err != nil {
					panic(err)
				}
				signatures = append(signatures, signature)
			}

			// complete the tx by adding its signature
			// (no network, no private key needed)
			err = tx.AddSignatures(signatures...)
			if err != nil {
				return fmt.Errorf("could not add signature(s): %v", err)
			}

			// submit the tx, wait a bit, fetch the tx info
			// (network needed)
			err = client.SubmitTx(context.Background(), tx)
			if err != nil {
				return fmt.Errorf("could not broadcast: %v", err)
			}
			logrus.WithField("hash", tx.Hash()).Info("submitted tx")
			start := time.Now()
			for time.Since(start) < timeout {
				time.Sleep(5 * time.Second)
				info, err := client.FetchTxInfo(context.Background(), tx.Hash())
				if err != nil {
					logrus.WithField("hash", tx.Hash()).WithError(err).Info("could not find tx on chain yet, trying again...")
					continue
				}
				fmt.Println(asJson(info))
				return nil
			}

			return fmt.Errorf("could not find transaction that we submitted by hash %s", tx.Hash())
		},
	}
	cmd.Flags().String("contract", "", "contract address of asset to send, if applicable")
	cmd.Flags().String("decimals", "", "decimals of the token, when using --contract.")
	cmd.Flags().String("memo", "", "set a memo for the transfer.")
	cmd.Flags().Duration("timeout", 1*time.Minute, "Amount of time to wait for transaction to confirm on chain.")
	return cmd
}

func CmdAddress() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "address",
		Short: fmt.Sprintf("Derive an address from the %s environment variable.", signer.EnvPrivateKey),
		Args:  cobra.ExactArgs(0),
		RunE: func(cmd *cobra.Command, args []string) error {
			xcFactory := setup.UnwrapXc(cmd.Context())
			chainConfig := setup.UnwrapChain(cmd.Context())

			privateKeyInput := signer.ReadPrivateKeyEnv()
			if privateKeyInput == "" {
				return fmt.Errorf("must set env %s", signer.EnvPrivateKey)
			}

			signer, err := xcFactory.NewSigner(chainConfig, privateKeyInput)
			if err != nil {
				return fmt.Errorf("could not import private key: %v", err)
			}

			publicKey, err := signer.PublicKey()
			if err != nil {
				return fmt.Errorf("could not create public key: %v", err)
			}

			addressBuilder, err := xcFactory.NewAddressBuilder(chainConfig)
			if err != nil {
				return fmt.Errorf("could not create address builder: %v", err)
			}

			from, err := addressBuilder.GetAddressFromPublicKey(publicKey)
			if err != nil {
				return fmt.Errorf("could not derive address: %v", err)
			}

			fmt.Println(from)

			return nil
		},
	}
	return cmd
}

func CmdChains() *cobra.Command {
	format := ""
	cmd := &cobra.Command{
		Use:   "chains",
		Short: "List information on all supported chains.",
		Args:  cobra.ExactArgs(0),
		RunE: func(cmd *cobra.Command, args []string) error {
			xcFactory := setup.UnwrapXc(cmd.Context())
			chain := setup.UnwrapChain(cmd.Context())

			cli, err := xcFactory.NewClient(assetConfig(chain, "", 0))
			if err != nil {
				return err
			}

			printer := func(data any) error {
				dataYamlBz, _ := yaml.Marshal(data)
				if format == "json" {
					reserialized := []interface{}{}
					err = yaml.Unmarshal(dataYamlBz, &reserialized)
					if err != nil {
						reserialized2 := map[string]interface{}{}
						err = yaml.Unmarshal(dataYamlBz, &reserialized2)
						fmt.Println(asJson(reserialized2))
						if err != nil {
							panic(err)
						}
					} else {
						fmt.Println(asJson(reserialized))
					}
				} else if format == "yaml" {
					fmt.Println(string(dataYamlBz))
				} else {
					return fmt.Errorf("invalid format")
				}
				return nil
			}

			if xccli, ok := cli.(*crosschain.Client); ok {
				logrus.Info("listing from remote configuration")
				apiURL := fmt.Sprintf("%s/v1/chains", xccli.URL)
				res, err := xccli.ApiCallWithUrl(context.Background(), "GET", apiURL, nil)
				if err != nil {
					return err
				}
				var data any
				json.Unmarshal(res, &data)
				err = printer(data)
				if err != nil {
					return err
				}
			} else {
				logrus.Info("listing from local configuration")
				chains := []*xc.ChainConfig{}
				for _, chain := range xcFactory.GetAllChains() {
					chain.Migrate()
					chains = append(chains, chain)
				}
				err = printer(chains)
				if err != nil {
					return err
				}
			}

			return nil
		},
	}
	cmd.Flags().StringVar(&format, "format", "json", "Format may be json or yaml")
	return cmd
}

func CmdDecimals() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "decimals",
		Short: "Lookup the configured decimals for an asset.",
		Args:  cobra.ExactArgs(0),
		RunE: func(cmd *cobra.Command, args []string) error {
			contract, err := cmd.Flags().GetString("contract")
			if err != nil {
				return err
			}
			xcFactory := setup.UnwrapXc(cmd.Context())
			chainConfig := setup.UnwrapChain(cmd.Context())

			client, err := xcFactory.NewClient(assetConfig(chainConfig, contract, 0))
			if err != nil {
				return err
			}
			clientWithDecimals, ok := client.(xcclient.ClientWithDecimals)
			if !ok {
				return fmt.Errorf("not implemented for %s", chainConfig.Chain)
			}

			// address := xcFactory.MustAddress(chainConfig, addressRaw)
			decimals, err := clientWithDecimals.FetchDecimals(context.Background(), xc.ContractAddress(contract))
			if err != nil {
				return fmt.Errorf("could not fetch decimals for %s: %v", contract, err)
			}

			fmt.Println(decimals)

			return nil
		},
	}
	cmd.Flags().String("contract", "", "Contract to use to query.")
	return cmd
}

func CmdFund() *cobra.Command {
	var contract string
	var amountHuman string
	var decimalsStr string
	var api string
	cmd := &cobra.Command{
		Use:   "fund [address]",
		Short: "Request funds from a crosschain node faucet.",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			xcFactory := setup.UnwrapXc(cmd.Context())
			chainConfig := setup.UnwrapChain(cmd.Context())
			address, err := inputAddressOrDerived(xcFactory, chainConfig, args)
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
	return cmd
}

func assetConfig(chain *xc.ChainConfig, contractMaybe string, decimals int32) xc.ITask {
	if contractMaybe != "" {
		token := xc.TokenAssetConfig{
			Contract:    contractMaybe,
			Chain:       chain.Chain,
			ChainConfig: chain,
			Decimals:    decimals,
		}
		return &token
	} else {
		return chain
	}
}
