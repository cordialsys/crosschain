//go:build !not_ci

package ci

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"testing"

	xc "github.com/cordialsys/crosschain"
	"github.com/cordialsys/crosschain/factory"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
)

var (
	chain         string
	rpc           string
	network       string
	algorithm     string
	contract      string
	decimalsStr   string
	feePayer      bool
	decimalsInput *int
)

func init() {
	flag.StringVar(&chain, "chain", "", "Used Blockchain chain")
	flag.StringVar(&rpc, "rpc", "", "RPC endpoint")
	flag.StringVar(&network, "network", "", "Bitcoin network, if relevant")
	flag.StringVar(&algorithm, "algorithm", "", "Used to override signature algorithm. Bitcoin only")
	flag.StringVar(&contract, "contract", "", "Contract address for token")
	flag.StringVar(&decimalsStr, "decimals", "", "Decimals used for token")
	flag.BoolVar(&feePayer, "fee-payer", false, "Use fee payer for transactions")

	logrus.SetLevel(logrus.DebugLevel)
}

func validateCLIInputs(t *testing.T) {
	if chain == "" {
		t.Fatal("--chain is required")
	}
	if rpc == "" {
		t.Fatal("--rpc is required")
	}
	if decimalsStr != "" {
		asInt, err := strconv.Atoi(decimalsStr)
		if err != nil {
			panic(err)
		}
		decimalsInput = &asInt
	}
}

func fundWallet(t *testing.T, chainConfig *xc.ChainConfig, walletAddress xc.Address, amount string, contractMaybe string, decimals int32) {
	require.NotNil(t, chainConfig)

	amountHuman, err := xc.NewAmountHumanReadableFromStr(amount)
	require.NoError(t, err)
	amountBlockchain := amountHuman.ToBlockchain(int32(decimals))

	fmt.Printf("funding wallet %s with %s %s\n", walletAddress, amountBlockchain, contractMaybe)

	// The RPC host is the same as the faucet host
	parsedURL, err := url.Parse(chainConfig.URL)
	if err != nil {
		panic(err)
	}

	host := parsedURL.Hostname()
	require.NotEmpty(t, host)

	assetId := string(chainConfig.Chain)
	if contractMaybe != "" {
		assetId = contractMaybe
	}

	faucetUrl := fmt.Sprintf("http://%s:10001/chains/%s/assets/%s", host, chainConfig.Chain, assetId)
	require.NoError(t, err)

	err = getTestTokensFromFaucet(faucetUrl, walletAddress, amountBlockchain)
	require.NoError(t, err)

}

func asJson(data any) string {
	bz, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		panic(err)
	}
	return string(bz)
}

func getTestTokensFromFaucet(faucetUrl string, walletAddress xc.Address, amount xc.AmountBlockchain) error {
	requestBody := map[string]interface{}{
		"amount":  amount.String(),
		"address": walletAddress,
	}
	requestBodyBz, err := json.Marshal(requestBody)
	if err != nil {
		return fmt.Errorf("error creating request body: %v", err)
	}
	fmt.Println("POST ", faucetUrl)
	fmt.Println(asJson(requestBody))

	req, err := http.NewRequest("POST", faucetUrl, bytes.NewBuffer(requestBodyBz))
	if err != nil {
		return fmt.Errorf("error creating request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("error sending request: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("error reading response: %v", err)
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status code: %d, body: %s", resp.StatusCode, string(body))
	}

	fmt.Printf("Response: %s\n", string(body))
	return nil
}

func deriveAddress(t *testing.T, xcFactory *factory.Factory, chainConfig *xc.ChainConfig, privateKey string) xc.Address {
	signer, err := xcFactory.NewSigner(chainConfig.Base(), privateKey)
	require.NoError(t, err)

	publicKey, err := signer.PublicKey()
	require.NoError(t, err)

	addressBuilder, err := xcFactory.NewAddressBuilder(chainConfig.Base())
	require.NoError(t, err)

	from, err := addressBuilder.GetAddressFromPublicKey(publicKey)
	require.NoError(t, err)

	return from
}
