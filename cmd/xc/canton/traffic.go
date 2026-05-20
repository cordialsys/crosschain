package canton

import (
	"context"
	"encoding/json"
	"fmt"

	xc "github.com/cordialsys/crosschain"
	cantonclient "github.com/cordialsys/crosschain/chain/canton/client"
	"github.com/cordialsys/crosschain/cmd/xc/setup"
	"github.com/spf13/cobra"
)

func CmdTraffic() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "traffic",
		Short: "Inspect queryable Canton traffic/top-up diagnostics",
		Args:  cobra.ExactArgs(0),
		RunE: func(cmd *cobra.Command, args []string) error {
			chainConfig := setup.UnwrapChain(cmd.Context())
			if chainConfig.Driver != xc.DriverCanton {
				return fmt.Errorf("canton traffic requires --chain CANTON, got driver %q", chainConfig.Driver)
			}

			xcFactory := setup.UnwrapXc(cmd.Context())
			rpcClient, err := xcFactory.NewClient(chainConfig)
			if err != nil {
				return err
			}
			client, ok := rpcClient.(*cantonclient.Client)
			if !ok {
				return fmt.Errorf("expected Canton local client, got %T", rpcClient)
			}

			inspection, err := client.InspectTraffic(context.Background())
			if err != nil {
				return err
			}
			bz, err := json.MarshalIndent(inspection, "", "  ")
			if err != nil {
				return err
			}
			fmt.Println(string(bz))
			return nil
		},
	}
	return cmd
}
