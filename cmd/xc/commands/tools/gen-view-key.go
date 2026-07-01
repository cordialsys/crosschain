package tools

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"

	"filippo.io/edwards25519"
	"github.com/spf13/cobra"
)

// CmdGenViewKey generates a Monero-compatible private view key: a random
// 32-byte Ed25519 scalar, canonically reduced mod L. Suitable for use as
// chain.view_key in the crosschain XMR config.
func CmdGenViewKey() *cobra.Command {
	cmd := &cobra.Command{
		Use:          "gen-view-key",
		Short:        "Generate a random private view key (32-byte Ed25519 scalar, hex-encoded).",
		Args:         cobra.ExactArgs(0),
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			var buf [64]byte
			if _, err := rand.Read(buf[:]); err != nil {
				return fmt.Errorf("read entropy: %w", err)
			}
			s, err := edwards25519.NewScalar().SetUniformBytes(buf[:])
			if err != nil {
				return fmt.Errorf("reduce scalar: %w", err)
			}
			fmt.Println(hex.EncodeToString(s.Bytes()))
			return nil
		},
	}
	return cmd
}
