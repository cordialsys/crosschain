package commands

import (
	"context"
	"encoding/json"
	"fmt"

	xc "github.com/cordialsys/crosschain"
	"github.com/cordialsys/crosschain/chain/crosschain"
	"github.com/cordialsys/crosschain/cmd/xc/setup"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

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
				err = json.Unmarshal(res, &data)
				if err != nil {
					return err
				}
				err = printer(data)
				if err != nil {
					return err
				}
			} else {
				logrus.Info("listing from local configuration")
				chains := []*xc.ChainConfig{}
				for _, chain := range xcFactory.GetAllChains() {
					chain.Configure(xcFactory.Config.HttpTimeout)
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
