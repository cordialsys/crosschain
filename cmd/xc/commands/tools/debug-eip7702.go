package tools

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/ethereum/go-ethereum/core/types"
	"github.com/spf13/cobra"
)

func CmdDebug() *cobra.Command {

	cmd := &cobra.Command{
		Use:     "debug-eip7702 <tx-data>",
		Aliases: []string{"debug-eip7702"},
		Short:   "Parse and print metadata for a raw eip7702 transaction.",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			txData := args[0]
			txBz, err := hex.DecodeString(strings.TrimPrefix(txData, "0x"))
			if err != nil {
				return fmt.Errorf("failed to decode hex data: %w", err)
			}

			tx := types.Transaction{}
			err = tx.UnmarshalBinary(txBz)
			if err != nil {
				return fmt.Errorf("failed to unmarshal tx data: %w", err)
			}

			r, s, v := tx.RawSignatureValues()
			hash := tx.Hash()

			fmt.Printf("Data: %s\n", hex.EncodeToString(tx.Data()))
			fmt.Printf("To: %s\n", tx.To().String())
			fmt.Printf("Cost: %s\n", tx.Cost().String())
			fmt.Printf("Gas: %d\n", tx.Gas())
			fmt.Printf("GasPrice: %s\n", tx.GasPrice().String())
			fmt.Printf("GasTipCap: %s\n", tx.GasTipCap().String())
			fmt.Printf("GasFeeCap: %s\n", tx.GasFeeCap().String())
			fmt.Printf("Nonce: %d\n", tx.Nonce())
			fmt.Printf("Value: %s\n", tx.Value().String())
			fmt.Printf("ChainID: %s\n", tx.ChainId().String())
			fmt.Printf("R: %s\n", r.String())
			fmt.Printf("S: %s\n", s.String())
			fmt.Printf("V: %d\n", v)
			fmt.Printf("Hash: %s\n", hash.String())
			for _, auth := range tx.SetCodeAuthorizations() {
				fmt.Printf("Auth.Address: %s\n", auth.Address.String())
				fmt.Printf("Auth.ChainID: %s\n", auth.ChainID.String())
				fmt.Printf("Auth.Nonce: %d\n", auth.Nonce)
				fmt.Printf("Auth.R: %s\n", auth.R.String())
				fmt.Printf("Auth.S: %s\n", auth.S.String())
				fmt.Printf("Auth.V: %d\n", auth.V)
			}
			bzJson, err := tx.MarshalJSON()
			if err != nil {
				return fmt.Errorf("failed to marshal tx: %w", err)
			}
			var m map[string]interface{}
			err = json.Unmarshal(bzJson, &m)
			if err != nil {
				return fmt.Errorf("failed to unmarshal tx: %w", err)
			}
			bzJsonIndent, err := json.MarshalIndent(m, "", "  ")
			if err != nil {
				return fmt.Errorf("failed to marshal tx: %w", err)
			}
			fmt.Printf("%s\n", string(bzJsonIndent))

			return nil
		},
	}
	return cmd
}
