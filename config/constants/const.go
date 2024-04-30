package constants

import (
	"os"
	"path/filepath"
)

const DefaultHomeEnv string = "CORDIAL_HOME"
const ConfigEnvOld string = "CORDIAL_CONFIG"
const ConfigEnv string = "CROSSCHAIN_CONFIG"

var DefaultHome string

func init() {
	if home := os.Getenv(DefaultHomeEnv); home != "" {
		DefaultHome = home
		return
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
