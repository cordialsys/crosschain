package config

import (
	"fmt"
)

type Secret string

func (s Secret) Load() (string, error) {
	return GetSecret(string(s))
}
func (s Secret) LoadOrBlank() string {
	deref, _ := GetSecret(string(s))
	return deref
}

func NewRawSecret(secret string) Secret {
	return Secret(fmt.Sprintf("raw:%s", secret))
}
