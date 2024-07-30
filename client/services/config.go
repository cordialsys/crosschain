package services

import (
	"os"

	xc "github.com/cordialsys/crosschain"
	"github.com/cordialsys/crosschain/config"
	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"
)

type KilnConfig struct {
	BaseUrl  string        `mapstructure:"base_url,omitempty" json:"base_url,omitempty" yaml:"base_url,omitempty" toml:"base_url,omitempty"`
	ApiToken config.Secret `mapstructure:"api_token,omitempty" json:"api_token,omitempty" yaml:"api_token,omitempty" toml:"api_token,omitempty"`
}
type TwinstakeConfig struct {
	BaseUrl string `mapstructure:"base_url,omitempty" json:"base_url,omitempty" yaml:"base_url,omitempty" toml:"base_url,omitempty"`

	Username string        `mapstructure:"username" json:"username" yaml:"username" toml:"username"`
	Password config.Secret `mapstructure:"password,omitempty" json:"password,omitempty" yaml:"password,omitempty" toml:"password,omitempty"`
	ClientId string        `mapstructure:"client_id" json:"client_id" yaml:"client_id" toml:"client_id"`
	Region   string        `mapstructure:"region" json:"region" yaml:"region" toml:"region"`
}

type ServicesConfig struct {
	Kiln      KilnConfig      `mapstructure:"kiln" json:"kiln" yaml:"kiln" toml:"kiln"`
	Twinstake TwinstakeConfig `mapstructure:"twinstake" json:"twinstake" yaml:"twinstake" toml:"twinstake"`
}

func DefaultConfig(network xc.NetworkSelector) *ServicesConfig {
	cfg := &ServicesConfig{
		Kiln: KilnConfig{
			BaseUrl:  "https://api.kiln.fi",
			ApiToken: "env:KILN_API_TOKEN",
		},
		Twinstake: TwinstakeConfig{
			BaseUrl:  "https://api.twinstake.io",
			Username: config.Secret("env:TWINSTAKE_USERNAME").LoadOrBlank(),
			Password: config.Secret("env:TWINSTAKE_PASSWORD"),
			ClientId: config.Secret("env:TWINSTAKE_CLIENT_ID").LoadOrBlank(),
			Region:   "eu-west-3", // reported default on twinstakes website
		},
	}
	if network == xc.NotMainnets {
		cfg.Kiln.BaseUrl = "https://api.testnet.kiln.fi"
		cfg.Twinstake.BaseUrl = "https://testnet.api.twinstake.io"
	}

	return cfg
}

func LoadConfig(network xc.NetworkSelector) (*ServicesConfig, error) {
	v := getViper()
	cfg := &ServicesConfig{}
	defaultCfg := DefaultConfig(network)
	err := config.RequireConfigWithViper(v, "", cfg, defaultCfg)
	if err != nil {
		return cfg, err
	}
	return cfg, nil
}

func LoadConfigFromFile(network xc.NetworkSelector, file string) (*ServicesConfig, error) {
	v := viper.New()
	v.SetConfigType("toml")
	v.SetConfigFile(file)
	cfg := &ServicesConfig{}
	defaultCfg := DefaultConfig(network)
	err := config.RequireConfigWithViper(v, "", cfg, defaultCfg)
	if err != nil {
		return cfg, err
	}
	return cfg, nil
}

const ConfigFileEnv = "STAKING_CONFIG"

func getViper() *viper.Viper {
	v := viper.New()
	v.SetConfigType("toml")
	if os.Getenv(ConfigFileEnv) != "" {
		logrus.WithField("config", os.Getenv(ConfigFileEnv)).Debug("loading staking configuration")
		v.SetConfigFile(os.Getenv(ConfigFileEnv))
	}
	return v
}
