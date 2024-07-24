package config

import (
	"context"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	secretmanager "cloud.google.com/go/secretmanager/apiv1"
	"cloud.google.com/go/secretmanager/apiv1/secretmanagerpb"
	"github.com/cordialsys/crosschain/config/constants"
	vault "github.com/hashicorp/vault/api"
	"github.com/spf13/viper"
	"google.golang.org/api/iterator"
	"gopkg.in/yaml.v3"
)

var noSuchFile = "no such file"
var notFoundIn = "not found in"

func getViper() *viper.Viper {
	// new instance of viper to avoid conflicts with, e.g., cosmos
	v := viper.New()
	// config file is config.yaml
	v.SetConfigName("config.yaml")
	v.SetConfigType("yaml")

	// If the config location env is set, use that.
	if os.Getenv(constants.ConfigEnv) != "" {
		v.SetConfigFile(os.Getenv(constants.ConfigEnv))
	} else {
		v.SetConfigFile(os.Getenv(constants.ConfigEnvOld))
	}

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
	return RequireConfigWithViper(v, section, unmarshalDst, defaults)
}

func RequireConfigWithViper(v *viper.Viper, section string, unmarshalDst interface{}, defaults interface{}) error {
	// config is where we store default values
	// panic if not available
	err := v.ReadInConfig()
	if err != nil {
		msg := strings.ToLower(err.Error())
		if defaults != nil && strings.Contains(msg, notFoundIn) {
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

type Secret string

func (s Secret) Load() (string, error) {
	return GetSecret(string(s))
}
func (s Secret) LoadOrBlank() string {
	deref, _ := GetSecret(string(s))
	return deref
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

	secretType := strings.ToLower(splits[0])
	args := strings.Split(strings.Join(splits[1:], ":"), ",")
	switch secretType {
	case "env":
		path := args[0]
		return strings.TrimSpace(os.Getenv(path)), nil
	case "file":
		path := args[0]
		if len(path) > 1 && path[0] == '~' {
			path = strings.Replace(path, "~", os.Getenv("HOME"), 1)
		}
		_, err := os.Stat(path)
		if err != nil {
			return "", err
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
		if len(args) != 2 {
			return "", errors.New("vault secret has 2 comma separated arguments (url,path)")
		}
		vaultUrl := args[0]
		vaultFullPath := args[1]

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
	case "gsm":
		// google secret manager
		if len(args) != 2 {
			return "", errors.New("gsm secret has 2 comma separated arguments (project,secret_name)")
		}
		project := args[0]
		if len(strings.Split(project, "/")) == 1 {
			// should have /projects/ prefix
			project = filepath.Join("projects", project)
		}
		name := args[1]

		client, err := secretmanager.NewClient(context.Background())
		if err != nil {
			return "", err
		}

		it := client.ListSecrets(context.Background(), &secretmanagerpb.ListSecretsRequest{
			Parent: project,
			Filter: name,
		})
		for {
			resp, err := it.Next()
			if err == iterator.Done {
				break
			}

			if err != nil {
				return "", err
			}

			_, secretName := filepath.Split(resp.Name)
			if secretName == name {
				// access the latest version
				latest := filepath.Join(resp.Name, "versions/latest")
				latestSecret, err := client.AccessSecretVersion(context.Background(), &secretmanagerpb.AccessSecretVersionRequest{
					Name: latest,
				})
				if err != nil {
					return "", err
				}
				return string(latestSecret.Payload.Data), nil
			}
		}
		return "", fmt.Errorf("could not find a gsm secret by name %s", name)
	case "raw":
		return strings.Join(splits[1:], ":"), nil
	}
	return "", errors.New("invalid secret source for: ***")
}
