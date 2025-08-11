package commands

import (
	"context"
	"encoding/hex"
	"fmt"

	xcdrivertypes "github.com/cordialsys/crosschain/chain/crosschain/types"
	"github.com/cordialsys/crosschain/cmd/xc/setup"
	"github.com/spf13/cobra"
)

func CmdRpcSubmit() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "submit [hex-encoded-tx]",
		Aliases: []string{"broadcast"},
		Short:   "Broadcast a serialized signed transaction.",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			xcFactory := setup.UnwrapXc(cmd.Context())
			chainConfig := setup.UnwrapChain(cmd.Context())

			payloadHex := args[0]
			payload, err := hex.DecodeString(payloadHex)
			if err != nil {
				return fmt.Errorf("could not decode payload: %v", err)
			}

			binaryTx := xcdrivertypes.NewBinaryTx(payload, nil)

			rpcClient, err := xcFactory.NewClient(chainConfig)
			if err != nil {
				return err
			}

			err = rpcClient.SubmitTx(context.Background(), binaryTx)
			if err != nil {
				return fmt.Errorf("could not submit tx: %v", err)
			}
			return nil
		},
	}
	return cmd
}
