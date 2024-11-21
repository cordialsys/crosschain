package commands

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"time"

	xc "github.com/cordialsys/crosschain"
	"github.com/cordialsys/crosschain/chain/crosschain"
	"github.com/cordialsys/crosschain/cmd/xc/setup"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

func CmdRpcBalance() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "balance <address>",
		Short: "Check balance of an asset.  Reported as big integer, not accounting for any decimals.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			contract, _ := cmd.Flags().GetString("contract")
			xcFactory := setup.UnwrapXc(cmd.Context())
			chainConfig := setup.UnwrapChain(cmd.Context())
			addressRaw := args[0]

			client, err := xcFactory.NewClient(AssetConfig(chainConfig, contract, 0))
			if err != nil {
				return err
			}

			address := xcFactory.MustAddress(chainConfig, addressRaw)
			balance, err := RetrieveBalance(client, address)

			fmt.Println(balance)

			return nil
		},
	}
	cmd.Flags().String("contract", "", "Contract to use to query.  Default will use the native asset to query.")
	return cmd
}

func CmdTxInput() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "tx-input <address>",
		Aliases: []string{"input"},
		Short:   "Check inputs for a new transaction.",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			addressTo, _ := cmd.Flags().GetString("to")
			contract, _ := cmd.Flags().GetString("contract")
			xcFactory := setup.UnwrapXc(cmd.Context())
			chainConfig := setup.UnwrapChain(cmd.Context())
			addressRaw := args[0]

			client, err := xcFactory.NewClient(AssetConfig(chainConfig, contract, 0))
			if err != nil {
				return err
			}

			fromAddress := xcFactory.MustAddress(chainConfig, addressRaw)
			toAddress := xcFactory.MustAddress(chainConfig, addressTo)

			txInput, err := RetrieveTxInput(client, fromAddress, toAddress)
			if err != nil {
				return err
			}

			fmt.Println(txInput)

			return nil
		},
	}
	cmd.Flags().String("contract", "", "Optional contract of token asset")
	cmd.Flags().String("to", "", "Optional destination address")
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

			client, err := xcFactory.NewClient(AssetConfig(chainConfig, "", 0))
			if err != nil {
				return err
			}

			txInfo, err := RetrieveTxInfo(client, hash)
			if err != nil {
				return err
			}

			fmt.Println(txInfo)

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

			privateKeyInput := os.Getenv("PRIVATE_KEY")
			if privateKeyInput == "" {
				return fmt.Errorf("must set env PRIVATE_KEY")
			}

			client, err := xcFactory.NewClient(AssetConfig(chainConfig, contract, decimals))
			if err != nil {
				return fmt.Errorf("could not load client: %v", err)
			}

			txTransfer, err := RetrieveTxTransfer(xcFactory, chainConfig, contract, memo, timeout, toWalletAddress, transferredAmount, decimals, privateKeyInput, client)
			if err != nil {
				return err
			}

			fmt.Println(txTransfer)

			return nil
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
		Short: "Derive an address from the PRIVATE_KEY environment variable.",
		Args:  cobra.ExactArgs(0),
		RunE: func(cmd *cobra.Command, args []string) error {
			xcFactory := setup.UnwrapXc(cmd.Context())
			chainConfig := setup.UnwrapChain(cmd.Context())

			privateKeyInput := os.Getenv("PRIVATE_KEY")
			if privateKeyInput == "" {
				return fmt.Errorf("must set env PRIVATE_KEY")
			}

			from, err := DeriveAddress(xcFactory, chainConfig, privateKeyInput)
			if err != nil {
				return err
			}

			fmt.Println(from)

			return nil
		},
	}
	return cmd
}

func CmdChains() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "chains",
		Short: "List information on all supported chains.",
		Args:  cobra.ExactArgs(0),
		RunE: func(cmd *cobra.Command, args []string) error {
			xcFactory := setup.UnwrapXc(cmd.Context())
			chain := setup.UnwrapChain(cmd.Context())

			cli, err := xcFactory.NewClient(AssetConfig(chain, "", 0))
			if err != nil {
				return err
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
				res, _ = json.MarshalIndent(data, "", "  ")

				fmt.Println(string(res))
			} else {
				logrus.Info("listing from local configuration")
				chains := []*xc.ChainConfig{}
				for _, asset := range xcFactory.GetAllAssets() {
					if chain, ok := asset.(*xc.ChainConfig); ok {
						chains = append(chains, chain)
					}
				}
				chainsBz, _ := json.MarshalIndent(chains, "", "  ")
				fmt.Println(string(chainsBz))
			}

			return nil
		},
	}
	return cmd
}
