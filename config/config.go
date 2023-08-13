package config

import (
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"strings"

	vault "github.com/hashicorp/vault/api"
	"github.com/jumpcrypto/crosschain/config/constants"
	"github.com/spf13/viper"
	"gopkg.in/yaml.v3"
)

var noSuchFile = "no such file"
var notFoundIn = "not found in"

func getViper() *viper.Viper {
	// new instance of viper to avoid conflicts with, e.g., cosmos
	v := viper.New()
	// config file is config.yaml
	v.SetConfigName("config")
	v.SetConfigType("yaml")

	// If the config location env is set, use that.
	v.SetConfigFile(os.Getenv(constants.ConfigEnv))

	// otherwise, prioritize current path or parent
	v.AddConfigPath(".")
	v.AddConfigPath("..")
	// Lastly, check home dir
	v.AddConfigPath(constants.DefaultHome)

	return v
}

// Load configuration.
// 1. Read in a configuration file based on environment variables and current path.
// 2. If a section is provided, e.g. "crosschain", then only that section will be treated as root and deserialized.
// 3. You may optionally provide an existing configuration object with default values.
// 4. If defaults are provided, an error will _not_ be returned if no config is found.
func RequireConfig(section string, unmarshalDst interface{}, defaults interface{}) error {
	v := getViper()
	// config is where we store default values
	// panic if not available
	err := v.ReadInConfig()
	if err != nil {
		msg := strings.ToLower(err.Error())
		if defaults != nil && (strings.Contains(msg, noSuchFile) || strings.Contains(msg, notFoundIn)) {
			// use the defaults by serializing and deserializing
			bz, err := yaml.Marshal(defaults)
			if err != nil {
				return err
			}
			return yaml.Unmarshal(bz, unmarshalDst)
		} else {
			return fmt.Errorf("fatal error reading config file: %w", err)
		}
	}
	// retrieve config
	if section != "" {
		// viper does not support partial deserialization so we
		// have to re-serialize and parse again
		asMap := v.GetStringMap(section)
		bz, _ := yaml.Marshal(asMap)
		err = yaml.Unmarshal(bz, unmarshalDst)
	} else {
		err = v.Unmarshal(unmarshalDst)
	}
	if err != nil {
		return err
	}

	if defaults != nil {
		return ApplyDefaults(defaults, unmarshalDst, unmarshalDst)
	} else {
		return nil
	}
}

func newVaultClient(cfg *vault.Config) (VaultLoader, error) {
	cli, err := vault.NewClient(cfg)
	if err != nil {
		return &DefaultVaultLoader{}, err
	}
	return &DefaultVaultLoader{Client: cli}, nil
}

var NewVaultClient = newVaultClient

type DefaultVaultLoader struct {
	*vault.Client
}

var _ VaultLoader = &DefaultVaultLoader{}

func (v *DefaultVaultLoader) LoadSecretData(vaultPath string) (*vault.Secret, error) {
	secret, err := v.Logical().Read(vaultPath)
	if err != nil || secret == nil { // yes, secret can be nil
		return &vault.Secret{}, err
	}
	return secret, nil
}

type VaultLoader interface {
	LoadSecretData(path string) (*vault.Secret, error)
}

// GetSecret returns a secret, e.g. from env variable. Extend as needed.
func GetSecret(uri string) (string, error) {
	value := uri

	splits := strings.Split(value, ":")
	if len(splits) < 2 {
		return "", errors.New("invalid secret source for: ***")
	}

	path := splits[1]
	switch key := splits[0]; key {
	case "env":
		return strings.TrimSpace(os.Getenv(path)), nil
	case "file":
		if len(path) > 1 && path[0] == '~' {
			path = strings.Replace(path, "~", os.Getenv("HOME"), 1)
		}
		file, err := os.Open(path)
		defer file.Close()
		if err != nil {
			return "", err
		}
		result, err := ioutil.ReadAll(file)
		if err != nil {
			return "", err
		}
		return strings.TrimSpace(string(result)), nil
	case "vault":
		vaultArgString := strings.Join(splits[1:], ":")
		vaultArgs := strings.Split(vaultArgString, ",")
		if len(vaultArgs) != 2 {
			return "", errors.New("vault secret has 2 comma separated arguments (url,path)")
		}
		// expect VAULT_TOKEN in env
		vaultUrl := vaultArgs[0]
		vaultFullPath := vaultArgs[1]

		cfg := &vault.Config{Address: vaultUrl}
		// just check the error
		_, err := vault.NewClient(cfg)
		if err != nil {
			return "", err
		}
		client, err := NewVaultClient(cfg)
		if err != nil {
			return "", err
		}

		idx := strings.LastIndex(vaultFullPath, "/")
		if idx == -1 || idx == len(vaultFullPath) { // idx shouldn't be the last char
			return "", errors.New("malformed vault secret in config file")
		}
		vaultKey := vaultFullPath[idx+1:]
		vaultPath := vaultFullPath[:idx]

		secret, err := client.LoadSecretData(vaultPath)
		if err != nil {
			return "", err
		}
		data, _ := secret.Data["data"].(map[string]interface{})
		result, _ := data[vaultKey].(string)
		return strings.TrimSpace(result), nil
	}
	return "", errors.New("invalid secret source for: ***")
}
