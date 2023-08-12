package constants

import (
	"os"
	"path/filepath"
)

var DefaultHomeEnv string = "CORDIAL_HOME"
var ConfigEnv string = "CORDIAL_CONFIG"
var DefaultHome string

func init() {
	if home := os.Getenv(DefaultHomeEnv); home != "" {
		DefaultHome = home
		return
	} else {
		// ~/.cordial default
		userHomeDir, err := os.UserHomeDir()
		if err != nil {
			panic(err)
		}
		DefaultHome = filepath.Join(userHomeDir, ".cordial")
	}
}
