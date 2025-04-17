package commands

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	xc "github.com/cordialsys/crosschain"
	xcaddress "github.com/cordialsys/crosschain/address"
	"github.com/cordialsys/crosschain/builder"
	"github.com/cordialsys/crosschain/chain/crosschain"
	"github.com/cordialsys/crosschain/client"
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
	signer, err := xcFactory.NewSigner(chainConfig.Base(), privateKeyInput)
	if err != nil {
		return "", fmt.Errorf("could not import private key: %v", err)
	}

	publicKey, err := signer.PublicKey()
	if err != nil {
		return "", fmt.Errorf("could not create public key: %v", err)
	}
	addressBuilder, err := xcFactory.NewAddressBuilder(chainConfig.Base())
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
	var decimal bool
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

			rpcClient, err := xcFactory.NewClient(chainConfig)
			if err != nil {
				return err
			}
			options := []client.GetBalanceOption{}
			if contract != "" {
				options = append(options, client.OptionContract(xc.ContractAddress(contract)))
			}
			balanceArgs := client.NewBalanceArgs(address, options...)

			balance, err := rpcClient.FetchBalance(context.Background(), balanceArgs)
			if err != nil {
				return fmt.Errorf("could not fetch balance for address %s: %v", address, err)
			}
			if decimal {
				if contract == "" {
					contract = string(chainConfig.Chain)
				}
				decimals, err := rpcClient.FetchDecimals(context.Background(), xc.ContractAddress(contract))
				if err != nil {
					return fmt.Errorf("could not fetch decimals for contract %s: %v", contract, err)
				}
				amount := balance.ToHuman(int32(decimals))
				fmt.Println(amount.String())
			} else {
				fmt.Println(balance.String())
			}

			return nil
		},
	}
	cmd.Flags().StringVar(&contract, "contract", "", "Optional contract of token asset")
	cmd.Flags().BoolVar(&decimal, "decimal", false, "Report balance as a decimal.  If set, the decimals will be looked up.")
	return cmd
}

func CmdTxInput() *cobra.Command {
	var addressTo string
	var amount string
	var contract string
	var publicKeyHex string
	var memo string
	var decimals int
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
			if contract != "" && decimals == -1 {
				return fmt.Errorf("must set --decimals if using --contract")
			}

			client, err := xcFactory.NewClient(chainConfig)
			if err != nil {
				return err
			}

			tfOptions := []builder.BuilderOption{}
			if contract != "" {
				tfOptions = append(tfOptions, builder.OptionContractAddress(xc.ContractAddress(contract)))
				tfOptions = append(tfOptions, builder.OptionContractDecimals(decimals))
			}
			if publicKeyHex != "" {
				publicKey, err := hex.DecodeString(strings.TrimPrefix(publicKeyHex, "0x"))
				if err != nil {
					return fmt.Errorf("could not decode public key: %v", err)
				}
				tfOptions = append(tfOptions, builder.OptionPublicKey(publicKey))
			} else {
				privateKeyInput := signer.ReadPrivateKeyEnv()
				if privateKeyInput != "" {
					signer, err := xcFactory.NewSigner(chainConfig.Base(), privateKeyInput)
					if err != nil {
						return fmt.Errorf("could not import private key: %v", err)
					}
					publicKey, err := signer.PublicKey()
					if err != nil {
						return fmt.Errorf("could not create public key: %v", err)
					}
					tfOptions = append(tfOptions, builder.OptionPublicKey(publicKey))
				}
			}
			if memo != "" {
				tfOptions = append(tfOptions, builder.OptionMemo(memo))
			}

			// default to smallest possible amount
			amountBlockchain := xc.NewAmountBlockchainFromUint64(1)
			if amount != "" {
				humanAmount, err := xc.NewAmountHumanReadableFromStr(amount)
				if err != nil {
					return fmt.Errorf("could parse amount: %v", err)
				}
				if decimals >= 0 {
					amountBlockchain = humanAmount.ToBlockchain(int32(decimals))
				} else {
					amountBlockchain = humanAmount.ToBlockchain(int32(chainConfig.GetDecimals()))
				}
			}

			tfArgs, err := builder.NewTransferArgs(
				fromAddress,
				xc.Address(addressTo),
				amountBlockchain,
				tfOptions...,
			)
			if err != nil {
				return fmt.Errorf("could not create transfer args: %v", err)
			}

			input, err := client.FetchTransferInput(context.Background(), tfArgs)
			if err != nil {
				return fmt.Errorf("could not fetch transaction input: %v", err)
			}

			fmt.Println(asJson(input))

			return nil
		},
	}
	cmd.Flags().StringVar(&contract, "contract", "", "Optional contract of token asset")
	cmd.Flags().IntVar(&decimals, "decimals", -1, "Optional decimals of the token asset")
	cmd.Flags().StringVar(&addressTo, "to", "", "Optional destination address")
	cmd.Flags().StringVar(&amount, "amount", "", "human amount to transfer")
	cmd.Flags().StringVar(&memo, "memo", "", "Optional memo for the transaction")
	cmd.Flags().StringVar(&publicKeyHex, "public-key", "",
		fmt.Sprintf("Public key in hex of the sender address (will use %s if set)", signer.EnvPrivateKey),
	)

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

			client, err := xcFactory.NewClient(chainConfig)
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
	var inclusiveFee bool
	var feePayer bool

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

			priorityStr, err := cmd.Flags().GetString("priority")
			if err != nil {
				return err
			}

			if decimalsStr == "" && contract != "" {
				return fmt.Errorf("must set --decimals if using --contract")
			}
			algorithm, _ := cmd.Flags().GetString("algorithm")
			addressArgs := []xcaddress.AddressOption{}
			if algorithm != "" {
				addressArgs = append(addressArgs, xcaddress.OptionAlgorithm(xc.SignatureType(algorithm)))
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

			client, err := xcFactory.NewClient(chainConfig)
			if err != nil {
				return fmt.Errorf("could not load client: %v", err)
			}

			transferredAmountHuman, err := xc.NewAmountHumanReadableFromStr(transferredAmount)
			if err != nil {
				return err
			}
			addressBuilder, err := xcFactory.NewAddressBuilder(chainConfig.Base(), addressArgs...)
			if err != nil {
				return fmt.Errorf("could not create address builder: %v", err)
			}

			amountBlockchain := transferredAmountHuman.ToBlockchain(decimals)
			tfOptions := []builder.BuilderOption{
				builder.OptionTimestamp(time.Now().Unix()),
			}

			mainSigner, err := xcFactory.NewSigner(chainConfig.Base(), privateKeyInput, addressArgs...)
			if err != nil {
				return fmt.Errorf("could not import private key: %v", err)
			}
			publicKey, err := mainSigner.PublicKey()
			if err != nil {
				return fmt.Errorf("could not create public key: %v", err)
			}
			tfOptions = append(tfOptions, builder.OptionPublicKey(publicKey))

			from, err := addressBuilder.GetAddressFromPublicKey(publicKey)
			if err != nil {
				return fmt.Errorf("could not derive address: %v", err)
			}
			signerCollection := signer.NewCollection()
			signerCollection.AddMainSigner(mainSigner, from)

			txBuilder, err := xcFactory.NewTxBuilder(chainConfig.GetChain().Base())
			if err != nil {
				return fmt.Errorf("could not load tx-builder: %v", err)
			}

			logrus.WithField("address", from).Info("sending from")
			if feePayer {
				_, ok := txBuilder.(builder.BuilderSupportsFeePayer)
				if !ok {
					return fmt.Errorf("support for fee payer on chain %s is not implemented", chainConfig.Chain)
				}
				feePayerPrivateKey := signer.ReadPrivateKeyFeePayerEnv()
				if feePayerPrivateKey == "" {
					return fmt.Errorf("must set env %s", signer.EnvPrivateKeyFeePayer)
				}
				feePayerSigner, err := xcFactory.NewSigner(chainConfig.Base(), feePayerPrivateKey)
				if err != nil {
					return fmt.Errorf("could not import fee-payer private key: %v", err)
				}
				feePayerPublicKey, err := feePayerSigner.PublicKey()
				if err != nil {
					return fmt.Errorf("could not create fee-payer public key: %v", err)
				}
				feePayerAddress, err := addressBuilder.GetAddressFromPublicKey(feePayerPublicKey)
				if err != nil {
					return fmt.Errorf("could not derive fee-payer address: %v", err)
				}
				logrus.WithField("fee-payer", feePayerAddress).Info("using fee-payer")
				tfOptions = append(tfOptions, builder.OptionFeePayer(feePayerAddress))
				signerCollection.AddAuxSigner(feePayerSigner, feePayerAddress)
			}

			if memo != "" {
				tfOptions = append(tfOptions, builder.OptionMemo(memo))
			}
			if contract != "" {
				tfOptions = append(tfOptions, builder.OptionContractAddress(xc.ContractAddress(contract)))
			}
			if decimalsStr != "" {
				tfOptions = append(tfOptions, builder.OptionContractDecimals(int(decimals)))
			}

			if priorityStr != "" {
				priority, err := xc.NewPriority(priorityStr)
				if err != nil {
					return fmt.Errorf("invalid priority: %v", err)
				}
				tfOptions = append(tfOptions, builder.OptionPriority(priority))
			}

			tfArgs, err := builder.NewTransferArgs(from, xc.Address(toWalletAddress), amountBlockchain, tfOptions...)
			if err != nil {
				return fmt.Errorf("invalid transfer args: %v", err)
			}

			// Get input from RPC
			input, err := client.FetchTransferInput(context.Background(), tfArgs)
			if err != nil {
				return fmt.Errorf("could not fetch transfer input: %v", err)
			}

			// set params on input that are enforced by the builder (rather than depending soley on untrusted RPC)
			input, err = builder.WithTxInputOptions(input, tfArgs.GetAmount(), &tfArgs)
			if err != nil {
				return fmt.Errorf("could not apply trusted options to tx-input: %v", err)
			}

			if inclusiveFee {
				fee, feeAssetId := input.GetFeeLimit()
				if contract != "" && feeAssetId != xc.ContractAddress(contract) {
					return fmt.Errorf("cannot include fee of asset %s in transfer of asset %s", feeAssetId, contract)
				}
				if contract == "" {
					if feeAssetId == "" {
						feeAssetId = xc.ContractAddress(chainConfig.Chain)
					}
					if feeAssetId != xc.ContractAddress(chainConfig.Chain) {
						return fmt.Errorf("cannot include fee of asset %s in transfer of asset %s", feeAssetId, chainConfig.Chain)
					}
				}
				amount := tfArgs.GetAmount()
				tfArgs.SetAmount(amount.Sub(&fee))
			}

			err = xc.CheckFeeLimit(input, chainConfig)
			if err != nil {
				return err
			}

			bz, _ := json.Marshal(input)
			logrus.WithField("input", string(bz)).Debug("transfer input")

			// create tx (no network, no private key needed)
			tx, err := txBuilder.Transfer(tfArgs, input)
			if err != nil {
				return fmt.Errorf("could not build transfer: %v", err)
			}

			// serialize tx for signing
			sighashes, err := tx.Sighashes()
			if err != nil {
				return fmt.Errorf("could not create payloads to sign: %v", err)
			}

			// sign
			signatures := []*xc.SignatureResponse{}
			for _, sighash := range sighashes {
				log := logrus.WithField("payload", hex.EncodeToString(sighash.Payload))
				// sign the tx sighash(es)
				signature, err := signerCollection.Sign(sighash.Signer, sighash.Payload)
				if err != nil {
					panic(err)
				}
				signatures = append(signatures, signature)
				log.
					WithField("address", signature.Address).
					WithField("signature", hex.EncodeToString(signature.Signature)).Info("adding signature")
			}

			// complete the tx by adding its signature
			// (no network, no private key needed)
			err = tx.AddSignatures(signatures...)
			if err != nil {
				return fmt.Errorf("could not add signature(s): %v", err)
			}

			// submit the tx, wait a bit, fetch the tx info (network needed)
			err = client.SubmitTx(context.Background(), tx)
			if err != nil {
				return fmt.Errorf("could not broadcast: %v", err)
			}
			logrus.WithField("hash", tx.Hash()).Info("submitted tx")

			time.Sleep(1 * time.Second)
			logrus.Info("fetching transaction...")
			start := time.Now()
			for time.Since(start) < timeout {
				info, err := client.FetchTxInfo(context.Background(), tx.Hash())
				if err != nil {
					logrus.WithField("hash", tx.Hash()).WithError(err).Info("could not find tx on chain yet, trying again in 3s...")
					time.Sleep(3 * time.Second)
					continue
				}

				if info.Confirmations < 1 {
					if logrus.GetLevel() >= logrus.DebugLevel {
						fmt.Fprintln(os.Stderr, asJson(info))
					}
					logrus.Info("waiting for confirmation...")
					time.Sleep(3 * time.Second)
					continue
				} else {
					fmt.Println(asJson(info))
					return nil
				}
			}

			return fmt.Errorf("could not find transaction that we submitted by hash %s", tx.Hash())
		},
	}
	cmd.Flags().String("contract", "", "Contract address of asset to send, if applicable")
	cmd.Flags().String("decimals", "", "Decimals of the token, when using --contract.")
	cmd.Flags().String("memo", "", "Set a memo for the transfer.")
	cmd.Flags().BoolVar(&feePayer, "fee-payer", false, "Use another address to pay the fee for the transaction (must set env PRIVATE_KEY_FEE_PAYER)")
	cmd.Flags().String("priority", "", "Apply a priority for the transaction fee ('low', 'market', 'aggressive', 'very-aggressive', or any positive decimal number)")
	cmd.Flags().Duration("timeout", 1*time.Minute, "Amount of time to wait for transaction to confirm on chain.")
	cmd.Flags().BoolVar(&inclusiveFee, "inclusive-fee", false, "Include the fee in the transfer amount.")
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
			algorithm, _ := cmd.Flags().GetString("algorithm")
			addressArgs := []xcaddress.AddressOption{}
			if algorithm != "" {
				addressArgs = append(addressArgs, xcaddress.OptionAlgorithm(xc.SignatureType(algorithm)))
			}

			privateKeyInput := signer.ReadPrivateKeyEnv()
			if privateKeyInput == "" {
				return fmt.Errorf("must set env %s", signer.EnvPrivateKey)
			}

			signer, err := xcFactory.NewSigner(chainConfig.Base(), privateKeyInput, addressArgs...)
			if err != nil {
				return fmt.Errorf("could not import private key: %v", err)
			}

			publicKey, err := signer.PublicKey()
			if err != nil {
				return fmt.Errorf("could not create public key: %v", err)
			}

			addressBuilder, err := xcFactory.NewAddressBuilder(chainConfig.Base(), addressArgs...)
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

			cli, err := xcFactory.NewClient(chain)
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
					chain.Configure()
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

			client, err := xcFactory.NewClient(chainConfig)
			if err != nil {
				return err
			}
			decimals, err := client.FetchDecimals(context.Background(), xc.ContractAddress(contract))
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
