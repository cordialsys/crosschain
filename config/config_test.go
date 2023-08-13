package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"

	vault "github.com/hashicorp/vault/api"
	"github.com/jumpcrypto/crosschain/config/constants"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type CrosschainTestSuite struct {
	suite.Suite
}

func (s *CrosschainTestSuite) SetupTest() {
}

func TestConfig(t *testing.T) {
	suite.Run(t, new(CrosschainTestSuite))
}

func (s *CrosschainTestSuite) TestRequireConfigNotFound() {
	require := s.Require()
	os.Setenv(constants.ConfigEnv, "___not_an_existing_file___")
	cfg := map[string]interface{}{}
	err := RequireConfig("crosschain", &cfg, nil)
	require.Error(err)
	require.ErrorContains(err, noSuchFile)

	// works if defaults are provided
	err = RequireConfig("crosschain", &cfg, &cfg)
	require.NoError(err)

	// different error if looking in directory for config file
	os.Unsetenv(constants.ConfigEnv)
	// change dir to somewhere that config files are not present
	os.Chdir(os.TempDir())
	os.Setenv(constants.DefaultHomeEnv, os.TempDir())
	err = RequireConfig("crosschain", &cfg, nil)
	require.Error(err)
	require.Contains(strings.ToLower(err.Error()), notFoundIn)

	// works if given defaults
	err = RequireConfig("crosschain", &cfg, &cfg)
	require.NoError(err)
}
func (s *CrosschainTestSuite) TestRequireConfigParsesYaml() {
	require := s.Require()
	file, err := os.CreateTemp(os.TempDir(), "xctest")
	require.NoError(err)
	file.Write([]byte(`
crosschain:
  chains:
    - asset: XYZ
    - asset: ABC
service2:
  stuff:
    - asset: 123
    - asset: 456
`))
	os.Setenv(constants.ConfigEnv, file.Name())
	defer os.Unsetenv(constants.ConfigEnv)

	cfg := map[string]interface{}{}
	err = RequireConfig("crosschain", &cfg, nil)
	require.NoError(err)
	require.Contains(cfg, "chains")
	require.NotContains(cfg, "service2")
	require.Len(cfg["chains"], 2)
	require.Equal("XYZ", cfg["chains"].([]interface{})[0].(map[string]interface{})["asset"])

	cfg = map[string]interface{}{}
	err = RequireConfig("service2", &cfg, nil)
	require.NoError(err)
	require.Contains(cfg, "stuff")
	require.NotContains(cfg, "chains")
	require.Len(cfg["stuff"], 2)
	require.Equal(123, cfg["stuff"].([]interface{})[0].(map[string]interface{})["asset"])

	cfg = map[string]interface{}{}
	err = RequireConfig("invalid", &cfg, nil)
	require.NoError(err)
	require.NotContains(cfg, "invalid")
	require.NotContains(cfg, "chains")
}

func (s *CrosschainTestSuite) TestGetSecretEnv() {
	require := s.Require()
	os.Setenv("XCTEST", "mysecret")
	secret, err := GetSecret("env:XCTEST")
	os.Unsetenv("XCTEST")
	require.Equal("mysecret", secret)
	require.Nil(err)
}

func (s *CrosschainTestSuite) TestGetSecretFile() {
	require := s.Require()
	secret, err := GetSecret("file:../LICENSE")
	require.Contains(secret, "Apache License")
	require.Nil(err)
}

func (s *CrosschainTestSuite) TestGetSecretFileHomeErrFileNotFound() {
	require := s.Require()
	secret, err := GetSecret("file:~/config-in-home")
	require.Equal("", secret)
	require.Error(err)
}

func (s *CrosschainTestSuite) TestGetSecretErrFileNotFound() {
	require := s.Require()
	secret, err := GetSecret("file:../LICENSEinvalid")
	require.Equal("", secret)
	require.Error(err)
}

func (s *CrosschainTestSuite) TestGetSecretErrNoColon() {
	require := s.Require()
	secret, err := GetSecret("invalid")
	require.Equal("", secret)
	require.Error(errors.New("invalid secret source for: ***"), err)
}

func (s *CrosschainTestSuite) TestGetSecretErrInvalidType() {
	require := s.Require()
	secret, err := GetSecret("invalid:value")
	require.Equal("", secret)
	require.Error(errors.New("invalid secret source for: ***"), err)
}

type MockedVaultLoaded struct {
	data map[string]interface{}
}

var _ VaultLoader = &MockedVaultLoaded{}

func (l *MockedVaultLoaded) LoadSecretData(path string) (*vault.Secret, error) {
	data, ok := l.data[path]
	if !ok {
		return &vault.Secret{}, errors.New("path not found")
	}
	return &vault.Secret{
		Data: data.(map[string]interface{}),
	}, nil
}

func (s *CrosschainTestSuite) TestGetSecretVault() {
	require := s.Require()
	NewVaultClient = func(cfg *vault.Config) (VaultLoader, error) {
		vaultRes := `{
			"path1/to": {
				"data": {
					"secret": "mysecret"
				}
			},
			"path2/to": {
				"data": {
					"secret2": "mysecret2"
				}
			}
		}`
		data := make(map[string]interface{})
		err := json.Unmarshal([]byte(vaultRes), &data)
		require.NoError(err)

		return &MockedVaultLoaded{
			data: data,
		}, nil
	}

	_, err := GetSecret("vault:wrong_args")
	require.ErrorContains(err, "vault secret has 2 comma separated arguments")
	_, err = GetSecret("vault:wrong_args,aaa,bbb")
	require.ErrorContains(err, "vault secret has 2 comma separated arguments")

	_, err = GetSecret("vault:url,aaa")
	require.ErrorContains(err, "malformed vault secret")

	_, err = GetSecret("vault:url,aaa/secret")
	require.EqualError(err, "path not found")

	secret, err := GetSecret("vault:https://example.com,path1/to/secret")
	require.NoError(err)
	require.Equal("mysecret", secret)

	secret, err = GetSecret("vault:https://example.com,path2/to/secret2")
	require.NoError(err)
	require.Equal("mysecret2", secret)

	secret, err = GetSecret("vault:https://example.com,path2/to/secret_none")
	require.NoError(err)
	require.Equal("", secret)
}

func (s *CrosschainTestSuite) TestGetSecretFileTrimmed() {
	require := s.Require()

	dir := os.TempDir()
	file, err := os.CreateTemp(dir, "config-test")
	require.NoError(err)
	defer file.Close()
	file.Write([]byte("MYSECRET"))
	file.Sync()

	sec, err := GetSecret("file:" + file.Name())
	require.NoError(err)
	require.Equal("MYSECRET", sec)

	file2, err := os.CreateTemp(dir, "config-test")
	require.NoError(err)
	defer file2.Close()
	// add whitespace
	file2.Write([]byte(" MY SECRET \n"))
	file2.Sync()

	sec, err = GetSecret("file:" + file2.Name())
	require.NoError(err)
	require.Equal("MY SECRET", sec)
}

type TestHobby struct {
	Type    string   `yaml:"type"`
	Actions []string `yaml:"actions"`
}
type TestFriend struct {
	Name     string                `yaml:"name"`
	Thoughts []string              `yaml:"thoughts,omitempty"`
	Hobbies  map[string]*TestHobby `yaml:"hobbies,omitempty"`
	Numbers  map[string]int        `yaml:"numbers,omitempty"`
}
type TestObj struct {
	Name      string                 `yaml:"name"`
	Age       int                    `yaml:"age"`
	Favorites []string               `yaml:"favorites"`
	Friends   map[string]*TestFriend `yaml:"friends"`
}

func WriteConfig(cfg string) {
	file, err := os.CreateTemp(os.TempDir(), "xctest")
	if err != nil {
		panic(err)
	}

	file.Write([]byte(cfg))
	os.Setenv(constants.ConfigEnv, file.Name())
}

func GetDefaults(require *require.Assertions) *TestObj {
	WriteConfig(`
test:
  name: "marley"
  age: 100
  favorites:
    - "trade"
    - "swap"
    - "bridge"
  friends:
    foo:
      name: "foo"
      thoughts:
        - "market"
        - "eth"
        - "coins"
      hobbies:
        running:
          type: exercise
          actions:
            - "left"
            - "right"
        drawing:
          type: art
          actions:
            - "blue"
            - "green"
      numbers:
        one: 1
        one2: 1
        seven: 7
    sol:
      name: "sol"
`)
	expected := TestObj{
		Name:      "marley",
		Age:       100,
		Favorites: []string{"trade", "swap", "bridge"},
		Friends: map[string]*TestFriend{
			"foo": {
				Name:     "foo",
				Thoughts: []string{"market", "eth", "coins"},
				Hobbies: map[string]*TestHobby{
					"running": {Type: "exercise", Actions: []string{"left", "right"}},
					"drawing": {Type: "art", Actions: []string{"blue", "green"}},
				},
				Numbers: map[string]int{
					"one":   1,
					"one2":  1,
					"seven": 7,
				},
			},
			"sol": {
				Name: "sol",
			},
		},
	}

	defaults := TestObj{}
	err := RequireConfig("test", &defaults, nil)
	require.NoError(err)
	require.Equal(expected, defaults)
	return &defaults
}

func (s *CrosschainTestSuite) TestUsingDefaults() {
	require := s.Require()

	type testcase struct {
		name      string
		cfg       string
		applyDiff func(cfg *TestObj)
	}

	for _, tc := range []testcase{
		{
			name: "overwrite name",
			cfg: `
test:
  name: "dave"
`,
			applyDiff: func(cfg *TestObj) {
				cfg.Name = "dave"
			},
		},
		{
			name: "test case for keys is lowercase",
			cfg: `
test:
  friends:
    SOL:
      name: SOL
`,
			applyDiff: func(cfg *TestObj) {
				cfg.Friends["sol"].Name = "SOL"
			},
		},
		{
			name: "overwrite nested name",
			cfg: `
test:
  friends:
    foo:
      name: "bar"
`,
			applyDiff: func(cfg *TestObj) {
				cfg.Friends["foo"].Name = "bar"
			},
		},

		{
			name: "add new element to map",
			cfg: `
  test:
    friends:
      new:
        name: "n"
        numbers:
          two: 2
`,
			applyDiff: func(cfg *TestObj) {
				cfg.Friends["new"] = &TestFriend{
					Name:    "n",
					Numbers: map[string]int{"two": 2},
				}
			},
		},
		{
			name: "empty map should not overwrite anything",
			cfg: `
test:
  friends:
`,
		},
		{
			name: "overriding number does not affect neighboring string",
			cfg: `
test:
  age: 200
`,
			applyDiff: func(cfg *TestObj) {
				cfg.Age = 200
			},
		},
		{
			name: "change very nested value",
			cfg: `
test:
  name: "eth"
  friends:
    foo:
      hobbies:
        running:
          type: "moving"
`,
			applyDiff: func(cfg *TestObj) {
				cfg.Name = "eth"
				cfg.Friends["foo"].Hobbies["running"].Type = "moving"
			},
		},
		{
			name: "overriding an array should not override anything",
			cfg: `
test:
  favorites:
    - "cats"
`,
		},
	} {
		fmt.Println("Config defaults test:", tc.name)
		defaults := GetDefaults(require)
		cfg := TestObj{}

		WriteConfig(tc.cfg)

		err := RequireConfig("test", &cfg, &defaults)
		require.NoError(err)

		if tc.applyDiff != nil {
			require.NotEqual(defaults, &cfg)
			tc.applyDiff(defaults)
			require.Equal(defaults, &cfg)
		} else {
			require.Equal(defaults, &cfg)
		}
	}

}
