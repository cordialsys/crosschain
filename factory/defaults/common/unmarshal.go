package common

import (
	"strings"

	xc "github.com/cordialsys/crosschain"
	"github.com/cordialsys/crosschain/factory/config"
	"github.com/sirupsen/logrus"
	"gopkg.in/yaml.v2"
)

type HasID interface {
	ID() xc.AssetID
}

// right now xc / viper will always lowercase the keys in maps.
// whereas unmarshaling "natively" will preserve case.
// So we need to do an extra step here to lowercase all of the keys
func lowercaseMap[T HasID](list map[string]T) map[string]T {
	toMap := map[string]T{}
	for _, item := range list {
		asset := strings.ToLower(string(item.ID()))
		if _, ok := toMap[asset]; ok {
			logrus.Warnf("multiple entries for %s (%T)", asset, item)
		}
		toMap[asset] = item
	}
	return toMap
}

func Unmarshal(data string) *config.Config {
	cfg := &config.Config{}
	err := yaml.Unmarshal([]byte(data), cfg)
	if err != nil {
		panic(err)
	}
	cfg.MigrateFields()

	cfg.Chains = lowercaseMap(cfg.Chains)

	return cfg
}
