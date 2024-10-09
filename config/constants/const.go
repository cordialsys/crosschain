package constants

import (
	"fmt"
	"os"
	"path/filepath"
)

const DefaultHomeEnv string = "TREASURY_HOME"
const ConfigEnvOld string = "CORDIAL_CONFIG"
const ConfigEnv string = "CROSSCHAIN_CONFIG"

var DefaultHome string

func init() {
	if home := os.Getenv(DefaultHomeEnv); home != "" {
		DefaultHome = home
		return
	} else if home := os.Getenv("CORDIAL_HOME"); home != "" {
		fmt.Fprintf(os.Stderr, "Warning: `CORDIAL_HOME` is deprecated, please rename to TREASURY_HOME\n")
		DefaultHome = home
	} else {
		// ~/.cordial default
		userHomeDir, err := os.UserHomeDir()
		if err != nil {
			DefaultHome = "/data"
		} else {
			DefaultHome = filepath.Join(userHomeDir, ".cordial")
		}
	}
}
