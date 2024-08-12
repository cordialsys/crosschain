package config

import (
	"fmt"
	"strings"
)

type Secret string

type SecretType string

var Env SecretType = "env"
var Vault SecretType = "vault"
var Raw SecretType = "raw"
var File SecretType = "file"
var GoogleSecretManager SecretType = "gsm"

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

func HasTypePrefix(secretRef string) bool {
	switch SecretType(strings.Split(secretRef, ":")[0]) {
	case Env, Vault, Raw, File, GoogleSecretManager:
		return true
	}
	return false
}
